package metal

import (
	"fmt"
	"github.com/metal-pod/metal-go/api/models"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"net"
)

type MatchExpression struct {
	Key      string   `json:"key" yaml:"key"`
	Operator string   `json:"operator" yaml:"operator"`
	Values   []string `json:"values" yaml:"values"`
}

type NodeSelector struct {
	MatchExpressions []*MatchExpression `json:"match-expressions" yaml:"match-expressions"`
}

type Peer struct {
	MyASN         string          `json:"my-asn" yaml:"my-asn"`
	ASN           string          `json:"peer-asn" yaml:"peer-asn"`
	Address       string          `json:"peer-address" yaml:"peer-address"`
	NodeSelectors []*NodeSelector `json:"node-selectors" yaml:"node-selectors"`
}

type AddressPool struct {
	Name      string   `json:"name" yaml:"name"`
	Protocol  string   `json:"protocol" yaml:"protocol"`
	Addresses []string `json:"addresses" yaml:"addresses"`
}

type MetallbConfig struct {
	Peers        []*Peer        `json:"peers" yaml:"peers"`
	AddressPools []*AddressPool `json:"address-pools" yaml:"address-pools"`
}

// syncMetalLBConfig synchronizes the metalLB config.
func (r *ResourcesController) syncMetalLBConfig() error {
	nodes, err := r.getNodes()
	if err != nil {
		return err
	}

	for _, n := range nodes {
		resp, err := machineByName(r.resources.client, types.NodeName(n.GetName()))
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		peers, err := createPeers(n, resp.Machine)
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		addressPools := createAddressPools(resp.Machine)

		config := &MetallbConfig{
			Peers:        peers,
			AddressPools: addressPools,
		}

		_ = config //TODO: Handle config
	}

	r.update(nodes)

	return nil
}

func createPeers(node *v1.Node, machine *models.V1MachineResponse) ([]*Peer, error) {
	if machine.Allocation == nil {
		return nil, fmt.Errorf("machine %q is not allocated", *machine.ID)
	}
	peer, err := createPeer(node, machine)
	if err != nil {
		return nil, err
	}
	peers := []*Peer{
		peer,
	}
	return peers, nil
}

func createPeer(node *v1.Node, machine *models.V1MachineResponse) (*Peer, error) {
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

	asn := fmt.Sprintf("%d", alloc.Networks[0].Asn)
	podCIDR := node.Spec.PodCIDR
	ip, _, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, err
	}

	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("cannot determine IP of CIDR %q", podCIDR)
	}
	address := string(ip[3] + 1)

	peer := &Peer{
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
	}

	return peer, nil
}

func createAddressPools(machine *models.V1MachineResponse) []*AddressPool {
	if machine.Allocation == nil || len(*machine.Allocation.Hostname) == 0 {
		return nil
	}
	addressPool := createAddressPool(machine.Allocation.Networks)
	addressPools := []*AddressPool{
		addressPool,
	}
	return addressPools
}

func createAddressPool(networks []*models.V1MachineNetwork) *AddressPool {
	var addresses []string
	for _, nw := range networks {
		if *nw.Primary {
			addresses = append(addresses, nw.Ips[0])
			continue
		}
		addresses = append(addresses, nw.Ips...)
	}

	addressPool := &AddressPool{
		Name:      "default",
		Protocol:  "bgp",
		Addresses: addresses,
	}
	return addressPool
}
