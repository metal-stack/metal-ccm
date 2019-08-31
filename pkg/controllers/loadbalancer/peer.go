package loadbalancer

import (
	"fmt"
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
	MyASN         int64           `json:"my-asn" yaml:"my-asn"`
	ASN           int64           `json:"peer-asn" yaml:"peer-asn"`
	Address       string          `json:"peer-address" yaml:"peer-address"`
	NodeSelectors []*NodeSelector `json:"node-selectors,omitempty" yaml:"node-selectors,omitempty"`
}

func NewPeer(hostname string, asn int64, cidr string) (*Peer, error) {
	matchExpression := &MatchExpression{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	gateway, err := computeGateway(cidr)
	if err != nil {
		return nil, err
	}

	return &Peer{
		MyASN:   asn,
		ASN:     asn,
		Address: gateway,
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
