package metal

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

func (pool *AddressPool) AppendIPs(ips ...string) {
	for _, ip := range ips {
		cidr := ip + "/32"

		if pool.ContainsCIDR(cidr) {
			continue
		}

		pool.CIDRs = append(pool.CIDRs, cidr)
	}
}
