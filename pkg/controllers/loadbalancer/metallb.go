package loadbalancer

import (
	"fmt"
	"strconv"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/tag"

	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/metal-stack/metal-go/api/models"

	"github.com/ghodss/yaml"
)

const (
	metallbNamespace     = "metallb-system"
	metallbConfigMapName = "config"
	metallbConfigMapKey  = "config"
)

// MetalLBConfig is a struct containing a config for metallb
type MetalLBConfig struct {
	Peers        []*Peer        `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func newMetalLBConfig() *MetalLBConfig {
	return &MetalLBConfig{}
}

// CalculateConfig computes the metallb config from given parameter input.
func (cfg *MetalLBConfig) CalculateConfig(ips []*models.V1IPResponse, nws map[string]*models.V1NetworkResponse, nodes []v1.Node) error {
	err := cfg.computeAddressPools(ips, nws)
	if err != nil {
		return err
	}
	err = cfg.computePeers(nodes)
	if err != nil {
		return err
	}
	return nil
}

func (cfg *MetalLBConfig) computeAddressPools(ips []*models.V1IPResponse, nws map[string]*models.V1NetworkResponse) error {
	for _, ip := range ips {
		nw, ok := nws[*ip.Networkid]
		if !ok {
			continue
		}
		if *nw.Underlay {
			continue
		}
		// we do not want IPs from networks where the parent networks are private
		if nw.Parentnetworkid != nil && *nw.Parentnetworkid != "" {
			parent, ok := nws[*nw.Parentnetworkid]
			if !ok {
				continue
			}
			if *parent.Privatesuper {
				continue
			}
		}
		net := *ip.Networkid
		cfg.addIPToPool(net, *ip)
	}
	return nil
}

func (cfg *MetalLBConfig) computePeers(nodes []v1.Node) error {
	cfg.Peers = []*Peer{} // we want an empty array of peers and not nil if there are no nodes
	for _, n := range nodes {
		labels := n.GetLabels()
		asnString, ok := labels[tag.MachineNetworkPrimaryASN]
		if !ok {
			return fmt.Errorf("node %q misses label: %s", n.GetName(), tag.MachineNetworkPrimaryASN)
		}
		asn, err := strconv.ParseInt(asnString, 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse valid integer from asn annotation: %v", err)
		}

		peer, err := newPeer(n, asn)
		if err != nil {
			klog.Infof("skipping peer: %v", err)
			continue
		}

		cfg.Peers = append(cfg.Peers, peer)
	}
	return nil
}

// Write inserts or updates the Metal-LB config.
func (cfg *MetalLBConfig) Write(client clientset.Interface) error {
	yaml, err := cfg.ToYAML()
	if err != nil {
		return nil
	}

	cm := make(map[string]string, 1)
	cm[metallbConfigMapKey] = yaml

	return kubernetes.ApplyConfigMap(client, metallbNamespace, metallbConfigMapName, cm)
}

// getOrCreateAddressPool returns the address pool of the given network.
// It will be created if it does not exist yet.
func (cfg *MetalLBConfig) getOrCreateAddressPool(poolName string, autoAssign bool) *AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := NewBGPAddressPool(poolName, autoAssign)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

// announceIPs appends the given IPs to the network address pools.
func (cfg *MetalLBConfig) addIPToPool(network string, ip models.V1IPResponse) {
	t := ip.Type
	poolType := metalgo.IPTypeEphemeral
	autoAssign := true
	if t != nil && *t == metalgo.IPTypeStatic {
		poolType = metalgo.IPTypeStatic
		autoAssign = false
	}
	poolName := fmt.Sprintf("%s-%s", network, poolType)
	pool := cfg.getOrCreateAddressPool(poolName, autoAssign)
	pool.AppendIP(*ip.Ipaddress)
}

// ToYAML returns this config in YAML format.
func (cfg *MetalLBConfig) ToYAML() (string, error) {
	bb, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}
