package loadbalancer

import (
	"fmt"
	"net/netip"

	"github.com/metal-stack/metal-lib/pkg/pointer"
)

const (
	bgpProtocol = "bgp"
)

type AddressPool struct {
	Name       string   `json:"name" yaml:"name"`
	Protocol   string   `json:"protocol" yaml:"protocol"`
	AutoAssign *bool    `json:"auto-assign" yaml:"auto-assign,omitempty"`
	CIDRs      []string `json:"addresses,omitempty" yaml:"addresses,omitempty"` // It is assumed that only Host addresses (/32 for ipv4 or /128 for ipv6) are used.
}

func NewBGPAddressPool(name string) *AddressPool {
	return &AddressPool{
		Name:       name,
		Protocol:   bgpProtocol,
		AutoAssign: pointer.Pointer(false),
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

func (pool *AddressPool) appendIP(ip string) error {
	parsed, err := netip.ParseAddr(ip)
	if err != nil {
		return err
	}
	var cidr string
	if parsed.Is4() {
		cidr = parsed.String() + "/32"
	} else if parsed.Is6() {
		cidr = parsed.String() + "/128"
	} else {
		return fmt.Errorf("unknown addressfamily of ip:%s", ip)
	}

	if pool.containsCIDR(cidr) {
		return nil
	}

	pool.CIDRs = append(pool.CIDRs, cidr)
	return nil
}

func (pool *AddressPool) String() string {
	return fmt.Sprintf("%s (%s): %v", pool.Name, pool.Protocol, pool.CIDRs)
}
