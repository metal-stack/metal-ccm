package loadbalancer

import (
	"fmt"
	"strings"

	"github.com/metal-pod/metal-ccm/pkg/resources/metallb"
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

func newPeer(peerAddressMap metallb.PeerAddressMap, hostname string, asn int64) (*Peer, error) {
	matchExpression := &MatchExpression{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	address, err := getPeerAddress(peerAddressMap, hostname)
	if err != nil {
		return nil, err
	}

	return &Peer{
		MyASN:   asn,
		ASN:     asn,
		Address: address,
		NodeSelectors: []*NodeSelector{
			{
				MatchExpressions: []*MatchExpression{
					matchExpression,
				},
			},
		},
	}, nil
}

func getPeerAddress(peerAddressMap metallb.PeerAddressMap, hostname string) (string, error) {
	for host, cidr := range peerAddressMap {
		if hostname == host {
			return cidr[:strings.Index(cidr, "/")], nil
		}
	}
	return "", fmt.Errorf("peer address for host %q could not be determined from calico IPAM blocks", hostname)
}
