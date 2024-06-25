package cilium

import (
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	v1 "k8s.io/api/core/v1"
)

type Peer struct {
	Peer         *loadbalancer.Peer   `json:"peer,omitempty" yaml:"peer,omitempty"`
	NodeSelector slimv1.LabelSelector `json:"node-selectors,omitempty" yaml:"node-selectors,omitempty"`
}

func newPeer(node v1.Node, asn int64) (*Peer, error) {
	hostname := node.GetName()

	matchExpression := slimv1.LabelSelectorRequirement{
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
		NodeSelector: slimv1.LabelSelector{
			MatchExpressions: []slimv1.LabelSelectorRequirement{
				matchExpression,
			},
		},
	}, nil
}
