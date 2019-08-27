package metal

import (
	"fmt"
	"github.com/metal-pod/metal-go/api/models"
	"net"
)

type MatchExpression struct {
	Key      string   `json:"key" yaml:"key"`
	Operator string   `json:"operator" yaml:"operator"`
	Values   []string `json:"values,omitempty" yaml:"values,omitempty"`
}

type NodeSelector struct {
	MatchExpressions []*MatchExpression `json:"match-expressions,omitempty" yaml:"match-expressions,omitempty"`
}

type Peer struct {
	MyASN         string          `json:"my-asn" yaml:"my-asn"`
	ASN           string          `json:"peer-asn" yaml:"peer-asn"`
	IP            string          `json:"peer-address" yaml:"peer-address"`
	NodeSelectors []*NodeSelector `json:"node-selectors,omitempty" yaml:"node-selectors,omitempty"`
}

func NewPeer(machine *models.V1MachineResponse, cidr string) (*Peer, error) {
	if machine.Allocation == nil {
		return nil, fmt.Errorf("machine %q is not allocated", *machine.ID)
	}

	alloc := machine.Allocation
	hostname := *alloc.Hostname
	if len(hostname) == 0 {
		return nil, fmt.Errorf("machine %q has no allocated hostname", *machine.ID)
	}

	if len(alloc.Networks) == 0 {
		return nil, fmt.Errorf("machine %q has no allocated networks", *machine.ID)
	}

	matchExpression := &MatchExpression{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	ip, err := computeGateway(cidr)
	if err != nil {
		return nil, err
	}

	asn := fmt.Sprintf("%d", alloc.Networks[0].Asn)

	return &Peer{
		MyASN: asn,
		ASN:   asn,
		IP:    ip,
		NodeSelectors: []*NodeSelector{
			{
				MatchExpressions: []*MatchExpression{
					matchExpression,
				},
			},
		},
	}, nil
}

func computeGateway(cidr string) (string, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	ip = ip.To4()
	if ip == nil {
		return "", fmt.Errorf("cannot determine IP of CIDR %q", cidr)
	}

	ip[3]++
	return ip.String(), nil
}
