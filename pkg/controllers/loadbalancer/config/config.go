package config

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LoadBalancerConfig interface {
	WriteCRs(ctx context.Context, c client.Client) error
}

type baseConfig struct {
	Peers        []*Peer        `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func New(loadBalancerType string, ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node, k8sClientSet clientset.Interface) (LoadBalancerConfig, error) {
	bc, err := newBaseConfig(ips, nws, nodes)
	if err != nil {
		return nil, err
	}

	switch loadBalancerType {
	case "metallb":
		return newMetalLBConfig(bc), nil
	case "cilium":
		return newCiliumConfig(bc, k8sClientSet), nil
	default:
		return newMetalLBConfig(bc), nil
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

func computeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) ([]*AddressPool, error) {
	var (
		pools []*AddressPool
		errs  []error
	)

	for _, ip := range ips {
		if !nws.Has(*ip.Networkid) {
			klog.Infof("skipping ip %q: not part of cluster networks", *ip.Ipaddress)
			continue
		}

		net := *ip.Networkid

		err := addIPToPool(pools, net, *ip)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return pools, nil
}

func computePeers(nodes []v1.Node) ([]*Peer, error) {
	var peers []*Peer

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

func addIPToPool(pools []*AddressPool, network string, ip models.V1IPResponse) error {
	poolType := models.V1IPBaseTypeEphemeral
	if pointer.SafeDeref(ip.Type) == models.V1IPBaseTypeStatic {
		poolType = models.V1IPBaseTypeStatic
	}

	var (
		poolName = fmt.Sprintf("%s-%s", strings.ToLower(network), poolType)
		pool     = getOrCreateAddressPool(pools, poolName)
	)

	return pool.appendIP(*ip.Ipaddress)
}

func getOrCreateAddressPool(pools []*AddressPool, poolName string) *AddressPool {
	for _, pool := range pools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := newBGPAddressPool(poolName)

	pools = append(pools, pool)

	return pool
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
