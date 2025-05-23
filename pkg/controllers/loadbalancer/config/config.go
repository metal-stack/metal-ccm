package config

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LoadBalancerType string

func LoadBalancerTypeFromString(lb string) (LoadBalancerType, error) {
	switch l := LoadBalancerType(lb); l {
	case LoadBalancerTypeCilium, LoadBalancerTypeMetalLB:
		return l, nil
	case LoadBalancerType(""): // our default if nothing  is specified is metallb
		return LoadBalancerTypeMetalLB, nil
	default:
		return LoadBalancerType(""), fmt.Errorf("unknown load balancer type: %s", lb)
	}
}

type LoadBalancerConfig interface {
	WriteCRs(ctx context.Context) error
}

type baseConfig struct {
	Peers        []*peer
	AddressPools addressPools
}

func New(loadBalancerType LoadBalancerType, ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node, c client.Client, k8sClientSet clientset.Interface) (LoadBalancerConfig, error) {
	bc, err := newBaseConfig(ips, nws, nodes)
	if err != nil {
		return nil, err
	}

	switch loadBalancerType {
	case LoadBalancerTypeMetalLB:
		return newMetalLBConfig(bc, c), nil
	case LoadBalancerTypeCilium:
		return newCiliumConfig(bc, c, k8sClientSet), nil
	default:
		return nil, fmt.Errorf("unknown load balancer type: %s", loadBalancerType)
	}
}

func newBaseConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) (*baseConfig, error) {
	pools, err := computeAddressPools(ips, nws)
	if err != nil {
		return nil, err
	}

	peers, err := computePeers(nodes)
	if err != nil {
		return nil, err
	}

	return &baseConfig{
		Peers:        peers,
		AddressPools: pools,
	}, nil
}

func computeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) (addressPools, error) {
	var (
		pools = addressPools{}
		errs  []error
	)

	for _, ip := range ips {
		if ip.Networkid == nil {
			return nil, fmt.Errorf("ip has no network id set: %s", pointer.SafeDeref(ip.Ipaddress))
		}

		if !nws.Has(*ip.Networkid) {
			klog.Infof("skipping ip %q: not part of cluster networks", *ip.Ipaddress)
			continue
		}

		var (
			net      = *ip.Networkid
			poolName = getPoolName(net, ip)
		)

		err := pools.addPoolIP(poolName, ip)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return pools, nil
}

func computePeers(nodes []v1.Node) ([]*peer, error) {
	var peers []*peer

	for _, n := range nodes {
		asn, err := getASNFromNodeLabels(n)
		if err != nil {
			return nil, err
		}

		peer, err := newPeer(n, asn)
		if err != nil {
			klog.Warningf("skipping peer: %v", err)
			continue
		}

		peers = append(peers, peer)
	}

	return peers, nil
}

func getASNFromNodeLabels(node v1.Node) (int64, error) {
	labels := node.GetLabels()

	asnString, ok := labels[tag.MachineNetworkPrimaryASN]
	if !ok {
		return 0, fmt.Errorf("node %q misses label: %s", node.GetName(), tag.MachineNetworkPrimaryASN)
	}

	asn, err := strconv.ParseInt(asnString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse valid integer from asn annotation: %w", err)
	}

	return asn, nil
}
