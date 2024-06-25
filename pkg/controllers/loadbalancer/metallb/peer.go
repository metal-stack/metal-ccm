package metallb

import (
	"fmt"

	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Peer struct {
	Peer          *loadbalancer.Peer     `json:"peer,omitempty" yaml:"peer,omitempty"`
	NodeSelectors []metav1.LabelSelector `json:"node-selectors,omitempty" yaml:"node-selectors,omitempty"`
}

func newPeer(node v1.Node, asn int64) (*Peer, error) {
	hostname := node.GetName()

	matchExpression := metav1.LabelSelectorRequirement{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	peer, err := loadbalancer.NewPeer(node, asn)
	if err != nil {
		return nil, err
	}

	return &Peer{
		Peer: peer,
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
