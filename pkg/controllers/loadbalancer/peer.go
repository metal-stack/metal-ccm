package loadbalancer

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

type Peer struct {
	MyASN   int64  `json:"my-asn" yaml:"my-asn"`
	ASN     int64  `json:"peer-asn" yaml:"peer-asn"`
	Address string `json:"peer-address" yaml:"peer-address"`
}

func NewPeer(node v1.Node, asn int64) (*Peer, error) {
	address, err := NodeAddress(node)
	if err != nil {
		return nil, err
	}
	return &Peer{
		MyASN:   asn,
		ASN:     asn,
		Address: address,
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
