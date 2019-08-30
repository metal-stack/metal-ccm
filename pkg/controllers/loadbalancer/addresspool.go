package loadbalancer

import "fmt"

const (
	bgpProtocol = "bgp"
)

type AddressPool struct {
	NetworkID string   `json:"name" yaml:"name"`
	Protocol  string   `json:"protocol" yaml:"protocol"`
	CIDRs     []string `json:"addresses,omitempty" yaml:"addresses,omitempty"` // It is assumed that only /32 addresses are used.
}

func NewBGPAddressPool(networkID string) *AddressPool {
	return &AddressPool{
		NetworkID: networkID,
		Protocol:  bgpProtocol,
	}
}

func (pool *AddressPool) ContainsCIDR(cidr string) bool {
	for _, CIDR := range pool.CIDRs {
		if cidr == CIDR {
			return true
		}
	}
	return false
}

func (pool *AddressPool) AppendIP(ip string) {
	cidr := ip + "/32"

	if pool.ContainsCIDR(cidr) {
		return
	}

	pool.CIDRs = append(pool.CIDRs, cidr)
}

func (pool *AddressPool) String() string {
	return fmt.Sprintf("%s (%s): %v", pool.NetworkID, pool.Protocol, pool.CIDRs)
}
