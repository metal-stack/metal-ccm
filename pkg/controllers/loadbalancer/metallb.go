package loadbalancer

import (
	"github.com/metal-pod/metal-ccm/pkg/resources/kubernetes"
	clientset "k8s.io/client-go/kubernetes"

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

// getPeer returns the peer of the given CIDR if existent.
func (cfg *MetalLBConfig) getPeer(cidr string) (*Peer, error) {
	ip, err := computeGateway(cidr)
	if err != nil {
		return nil, err
	}

	for _, p := range cfg.Peers {
		if p.Address == ip {
			return p, nil
		}
	}

	return nil, nil
}

// getOrCreateAddressPool returns the address pool of the given network.
// It will be created if it does not exist yet.
func (cfg *MetalLBConfig) getOrCreateAddressPool(networkID string) *AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.NetworkID == networkID {
			return pool
		}
	}

	pool := NewBGPAddressPool(networkID)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

// announceIPs appends the given IPs to the network address pools.
func (cfg *MetalLBConfig) addIPToPool(network string, ip string) {
	pool := cfg.getOrCreateAddressPool(network)
	pool.AppendIP(ip)
}

// ToYAML returns this config in YAML format.
func (cfg *MetalLBConfig) ToYAML() (string, error) {
	bb, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}

func (cfg *MetalLBConfig) StringAddressPools() string {
	result := ""
	for _, pool := range cfg.AddressPools {
		result += pool.String() + " "
	}
	return result
}
