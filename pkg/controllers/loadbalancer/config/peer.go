package config

import (
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type peer struct {
	MyASN        uint32
	ASN          uint32
	Address      string
	NodeSelector metav1.LabelSelector
}

func newPeer(node v1.Node, asn int64) (*peer, error) {
	hostname := node.GetName()

	matchExpression := metav1.LabelSelectorRequirement{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	address, err := kubernetes.NodeAddress(node)
	if err != nil {
		return nil, err
	}

	return &peer{
		// we can safely cast the asn to an uint32 because its max value is defined as such
		// see: https://en.wikipedia.org/wiki/Autonomous_system_(Internet)
		MyASN:   uint32(asn), // nolint:gosec
		ASN:     uint32(asn), // nolint:gosec
		Address: address,
		NodeSelector: metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				matchExpression,
			},
		},
	}, nil
}
