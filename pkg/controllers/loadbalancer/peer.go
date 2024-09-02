package loadbalancer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Peer struct {
	MyASN         uint32                 `json:"my-asn" yaml:"my-asn"`
	ASN           uint32                 `json:"peer-asn" yaml:"peer-asn"`
	Address       string                 `json:"peer-address" yaml:"peer-address"`
	NodeSelectors []metav1.LabelSelector `json:"node-selectors,omitempty" yaml:"node-selectors,omitempty"`
}

func newPeer(node v1.Node, asn uint32) (*Peer, error) {
	hostname := node.GetName()

	matchExpression := metav1.LabelSelectorRequirement{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	address, err := NodeAddress(node)
	if err != nil {
		return nil, err
	}
	return &Peer{
		MyASN:   asn,
		ASN:     asn,
		Address: address,
		NodeSelectors: []metav1.LabelSelector{
			{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					matchExpression,
				},
			},
		},
	}, nil
}

func NodeAddress(node v1.Node) (string, error) {
	for _, a := range node.Status.Addresses {
		if a.Type == v1.NodeInternalIP {
			return a.Address, nil
		}
	}
	return "", fmt.Errorf("unable to determine node address")
}
