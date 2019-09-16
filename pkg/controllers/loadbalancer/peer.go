package loadbalancer

import (
	"fmt"

	"github.com/metal-pod/metal-ccm/pkg/resources/constants"

	v1 "k8s.io/api/core/v1"
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

func newPeer(node v1.Node, asn int64) (*Peer, error) {
	hostname := node.GetName()

	matchExpression := &MatchExpression{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	address, err := getPeerAddress(node)
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

func getPeerAddress(node v1.Node) (string, error) {
	annotations := node.GetAnnotations()

	tunnelAddress, ok := annotations[constants.CalicoIPTunnelAddr]
	if !ok {
		return "", fmt.Errorf("unable to determine tunnel address, calico has not yet added a node annotation")
	}

	return tunnelAddress, nil
}
