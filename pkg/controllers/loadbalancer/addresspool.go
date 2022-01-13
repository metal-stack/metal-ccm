package loadbalancer

import "fmt"

const (
	bgpProtocol = "bgp"
)

type AddressPool struct {
	Name       string   `json:"name" yaml:"name"`
	Protocol   string   `json:"protocol" yaml:"protocol"`
	AutoAssign bool     `json:"auto-assign" yaml:"auto-assign"`
	CIDRs      []string `json:"addresses,omitempty" yaml:"addresses,omitempty"` // It is assumed that only /32 addresses are used.
}

func NewBGPAddressPool(name string) *AddressPool {
	return &AddressPool{
		Name:       name,
		Protocol:   bgpProtocol,
		AutoAssign: false,
	}
}

func (pool *AddressPool) containsCIDR(cidr string) bool {
	for _, CIDR := range pool.CIDRs {
		if cidr == CIDR {
			return true
		}
	}
	return false
}

func (pool *AddressPool) appendIP(ip string) {
	cidr := ip + "/32"

	if pool.containsCIDR(cidr) {
		return
	}

	pool.CIDRs = append(pool.CIDRs, cidr)
}

func (pool *AddressPool) String() string {
	return fmt.Sprintf("%s (%s): %v", pool.Name, pool.Protocol, pool.CIDRs)
}
