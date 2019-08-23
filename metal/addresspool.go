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

func (ap *AddressPool) ContainsIP(ip string) bool {
	for _, IP := range ap.IPs {
		if ip == IP {
			return true
		}
	}
	return false
}

func (ap *AddressPool) AppendIPs(ips ...string) {
	for _, ip := range ips {
		if ap.ContainsIP(ip) {
			continue
		}

		ap.IPs = append(ap.IPs, ip)
	}
}
