package metal

import (
	"github.com/ghodss/yaml"
	"github.com/metal-pod/metal-go/api/models"
)

type MetalLBConfig struct {
	Peers        []*Peer        `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func newMetalLBConfig() *MetalLBConfig {
	return &MetalLBConfig{}
}

// getPeer returns the peer of the given CIDR if existent.
func (cfg *MetalLBConfig) getPeer(cidr string) (*Peer, error) {
	ip, err := computeGateway(cidr)
	if err != nil {
		return nil, err
	}

	for _, p := range cfg.Peers {
		if p.IP == ip {
			return p, nil
		}
	}

	return nil, nil
}

// getAddressPool returns the address pool of the given network.
// It will be created if it does not exist yet.
func (cfg *MetalLBConfig) getAddressPool(networkID string) *AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.NetworkID == networkID {
			return pool
		}
	}

	pool := NewBGPAddressPool(networkID)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

// announceMachineIPs appends the allocated IPs of the given machine to their corresponding address pools.
func (cfg *MetalLBConfig) announceMachineIPs(machine *models.V1MachineResponse) {
	if machine.Allocation == nil {
		return
	}

	for _, nw := range machine.Allocation.Networks {
		if nw == nil || (nw.Private != nil && *nw.Private) || (nw.Underlay != nil && *nw.Underlay) {
			continue
		}

		cfg.announceIPs(*nw.Networkid, nw.Ips...)
	}
}

// announceIPs appends the given IPs to the network address pools.
func (cfg *MetalLBConfig) announceIPs(network string, ips ...string) {
	pool := cfg.getAddressPool(network)
	pool.AppendIPs(ips...)
}

// ToYAML returns this config in YAML format.
func (cfg *MetalLBConfig) ToYAML() (string, error) {
	bb, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}
