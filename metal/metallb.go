package metal

import (
	"encoding/json"
	"fmt"
	"github.com/metal-pod/metal-go/api/models"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"net"
)

const (
	metallbNamespace     = "metallb-system"
	metallbConfigMapName = "config"
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

	config := &MetallbConfig{}

	for _, node := range nodes {
		resp, err := machineByName(r.resources.client, types.NodeName(node.GetName()))
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		peer, err := createPeer(node, resp.Machine)
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		addressPool := createAddressPool(resp.Machine)

		config.Peers = append(config.Peers, peer)
		config.AddressPools = append(config.AddressPools, addressPool)
	}

	return r.updateMetalLBConfig(config)
}

// updateMetalLBConfig updates given metalLB config.
func (r *ResourcesController) updateMetalLBConfig(config *MetallbConfig) error {
	var configMap map[string]string
	marshalledConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}

	err = json.Unmarshal(marshalledConfig, &configMap)
	if err != nil {
		return err
	}

	return r.upsertConfigMap(metallbNamespace, metallbConfigMapName, configMap)
}

// upsertConfigMap inserts or updates given config map.
func (r *ResourcesController) upsertConfigMap(namespace, name string, configMap map[string]string) error {
	binaryConfigMap := make(map[string][]byte, len(configMap))
	for k, v := range configMap {
		binaryConfigMap[k] = []byte(v)
	}

	err := retry.RetryOnConflict(updateNodeSpecBackoff, func() error {
		cmi := r.kclient.CoreV1().ConfigMaps(namespace)
		cm, err := cmi.Get(name, metav1.GetOptions{})
		if err == nil {
			cm.Data = configMap
			cm.BinaryData = binaryConfigMap

			_, err = cmi.Update(cm)
			return err
		}

		cm = &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:                       name,
				Namespace:                  namespace,
				DeletionGracePeriodSeconds: nil,
				Labels:                     nil,
				Annotations:                nil,
			},
			Data:       configMap,
			BinaryData: binaryConfigMap,
		}

		_, err = cmi.Create(cm)
		return err
	})

	return err
}

func createPeer(node *v1.Node, machine *models.V1MachineResponse) (*Peer, error) {
	if machine.Allocation == nil {
		return nil, fmt.Errorf("machine %q is not allocated", *machine.ID)
	}

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

func createAddressPool(machine *models.V1MachineResponse) *AddressPool {
	if machine.Allocation == nil || len(*machine.Allocation.Hostname) == 0 {
		return nil
	}

	var addresses []string
	for _, nw := range machine.Allocation.Networks {
		if *nw.Primary {
			addresses = append(addresses, nw.Ips[0])
			continue
		}
		addresses = append(addresses, nw.Ips...)
	}

	return &AddressPool{
		Name:      "default",
		Protocol:  "bgp",
		Addresses: addresses,
	}
}
