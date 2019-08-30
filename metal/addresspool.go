package metal

const (
	bgpProtocol = "bgp"
)

type AddressPool struct {
	NetworkID string   `json:"name" yaml:"name"`
	Protocol  string   `json:"protocol" yaml:"protocol"`
	IPs       []string `json:"addresses,omitempty" yaml:"addresses,omitempty"`
}

func NewBGPAddressPool(networkID string) *AddressPool {
	return &AddressPool{
		NetworkID: networkID,
		Protocol:  bgpProtocol,
	}
}

func (pool *AddressPool) ContainsIP(ip string) bool {
	for _, IP := range pool.IPs {
		if ip == IP {
			return true
		}
	}
	return false
}

func (pool *AddressPool) AppendIPs(ips ...string) {
	for _, ip := range ips {
		if pool.ContainsIP(ip) {
			continue
		}

		pool.IPs = append(pool.IPs, ip)
	}
}
