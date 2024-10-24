package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/tag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LoadBalancerConfig interface {
	PrepareConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error
	WriteCRs(ctx context.Context, c client.Client) error
}

type Config struct {
	Peers        []*Peer        `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func (cfg *Config) ComputeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) error {
	var errs []error
	for _, ip := range ips {
		if !nws.Has(*ip.Networkid) {
			klog.Infof("skipping ip %q: not part of cluster networks", *ip.Ipaddress)
			continue
		}

		klog.Infof("adding ip to pool %s", *ip.Ipaddress)

		net := *ip.Networkid
		err := cfg.addIPToPool(net, *ip)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (cfg *Config) PrepareConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error {
	err := cfg.ComputeAddressPools(ips, nws)
	if err != nil {
		return err
	}
	err = cfg.computePeers(nodes)
	if err != nil {
		return err
	}
	return nil
}

func (cfg *Config) computePeers(nodes []v1.Node) error {
	cfg.Peers = []*Peer{} // we want an empty array of peers and not nil if there are no nodes
	for _, n := range nodes {
		asn, err := cfg.GetASNFromNodeLabels(n)
		if err != nil {
			return err
		}

		peer, err := newPeer(n, asn)
		if err != nil {
			klog.Warningf("skipping peer: %v", err)
			continue
		}

		cfg.Peers = append(cfg.Peers, peer)
	}
	return nil
}

func (cfg *Config) addIPToPool(network string, ip models.V1IPResponse) error {
	t := ip.Type
	poolType := models.V1IPBaseTypeEphemeral
	if t != nil && *t == models.V1IPBaseTypeStatic {
		poolType = models.V1IPBaseTypeStatic
	}
	poolName := fmt.Sprintf("%s-%s", strings.ToLower(network), poolType)
	pool := cfg.getOrCreateAddressPool(poolName)

	err := pool.appendIP(*ip.Ipaddress)
	if err != nil {
		return err
	}

	return nil
}

func (cfg *Config) getOrCreateAddressPool(poolName string) *AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := newBGPAddressPool(poolName)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

func (cfg *Config) GetASNFromNodeLabels(node v1.Node) (int64, error) {
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
