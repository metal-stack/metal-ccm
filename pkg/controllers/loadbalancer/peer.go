package loadbalancer

import (
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
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

type IPAMBlock struct {
	Spec IPAMBlockSpec `json:"spec"`
}

type IPAMBlockSpec struct {
	Affinity string `json:"affinity"`
	Cidr     string `json:"cidr"`
}

func newPeer(client dynamic.Interface, hostname string, asn int64) (*Peer, error) {
	matchExpression := &MatchExpression{
		Key:      "kubernetes.io/hostname",
		Operator: "In",
		Values: []string{
			hostname,
		},
	}

	address, err := getPeerAddress(client, hostname)
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

func getPeerAddress(client dynamic.Interface, hostname string) (string, error) {
	resource := client.Resource(schema.GroupVersionResource{Group: "crd.projectcalico.org", Version: "v1", Resource: "ipamblocks"})

	ipamBlocksRaw, err := resource.List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, ipamBlockRaw := range ipamBlocksRaw.Items {
		raw, err := json.Marshal(ipamBlockRaw.Object)
		if err != nil {
			return "", err
		}

		var ipamBlock IPAMBlock
		err = json.Unmarshal(raw, &ipamBlock)
		if err != nil {
			return "", err
		}

		if strings.HasSuffix(ipamBlock.Spec.Affinity, hostname) {
			cidr := ipamBlock.Spec.Cidr
			return cidr[:strings.Index(cidr, "/")], nil
		}
	}
	return "", fmt.Errorf("peer address for host %q could not be determined from calico IPAM blocks", hostname)
}
