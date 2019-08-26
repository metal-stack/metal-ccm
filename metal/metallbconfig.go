package metal

import (
	"encoding/json"
	"github.com/metal-pod/metal-go/api/models"
)

type Config struct {
	Peers        []*Peer        `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

type LB struct {
	Config *Config `json:"config,omitempty" yaml:"config,omitempty"`
}

func newLB() *LB {
	return &LB{
		Config: &Config{},
	}
}

// getPeer returns the peer of the given CIDR if existent.
func (mlb *LB) getPeer(cidr string) (*Peer, error) {
	ip, err := computeGateway(cidr)
	if err != nil {
		return nil, err
	}

	for _, p := range mlb.Config.Peers {
		if p.IP == ip {
			return p, nil
		}
	}

	return nil, nil
}

// getAddressPool returns the address pool of the given network.
// It will be created if it does not exist yet.
func (mlb *LB) getAddressPool(networkID string) *AddressPool {
	for _, pool := range mlb.Config.AddressPools {
		if pool.NetworkID == networkID {
			return pool
		}
	}

	pool := NewBGPAddressPool(networkID)
	mlb.Config.AddressPools = append(mlb.Config.AddressPools, pool)

	return pool
}

// announceMachineIPs appends the allocated IPs of the given machine to their corresponding address pools.
func (mlb *LB) announceMachineIPs(machine *models.V1MachineResponse) {
	if machine.Allocation == nil {
		return
	}

	for _, nw := range machine.Allocation.Networks {
		if nw == nil || (nw.Private != nil && *nw.Private) || (nw.Underlay != nil && *nw.Underlay) {
			continue
		}

		mlb.announceIPs(*nw.Networkid, nw.Ips...)
	}
}

// announceIPs appends the given IPs to the network address pools.
func (mlb *LB) announceIPs(network string, ips ...string) {
	pool := mlb.getAddressPool(network)
	pool.AppendIPs(ips...)
}

// Json return this config as a JSON byte array.
func (mlb *LB) Json() ([]byte, error) {
	bb, err := json.Marshal(mlb)
	if err != nil {
		return nil, err
	}
	return bb, nil
}

// String returns this config as a prettified JSON string.
func (mlb *LB) String() string {
	bb, err := json.MarshalIndent(mlb, "", "    ")
	if err != nil {
		return err.Error()
	}
	return string(bb)
}
