package metal

import (
	"github.com/metal-pod/metal-go/api/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	namespace     = "metallb-system"
	configMapName = "config"
	configMapKey  = "config"
)

// AddFirewallNetworkAddressPools creates and adds empty address pools for all non-private and non-underlay firewall networks.
func (r *ResourcesController) AddFirewallNetworkAddressPools(node *v1.Node) error {
	mm := r.resources.machines.getMachines(node)
	fw, err := r.firewallOfMachine(*mm[0].ID)
	if err != nil {
		return err
	}
	if fw == nil || fw.Allocation == nil {
		return nil
	}

	var networkIDs []string
	for _, nw := range fw.Allocation.Networks {
		if nw == nil || (nw.Private != nil && *nw.Private) || (nw.Underlay != nil && *nw.Underlay) || nw.Networkid == nil || len(*nw.Networkid) == 0 {
			continue
		}

		handled := false
		for _, id := range networkIDs {
			if id == *nw.Networkid {
				handled = true
				break
			}
		}

		if !handled {
			networkIDs = append(networkIDs, *nw.Networkid)
			r.metallbConfig.AddressPools = append(r.metallbConfig.AddressPools, NewBGPAddressPool(*nw.Networkid))
		}
	}

	return r.upsertMetalLBConfig()
}

func (r *ResourcesController) firewallOfMachine(machineID string) (*models.V1FirewallResponse, error) { //TODO provide metal-api endpoint, adjust and move to metal-go
	m, err := r.resources.client.MachineGet(machineID)
	if err != nil {
		return nil, err
	}

	if m == nil || m.Machine == nil || m.Machine.Allocation == nil {
		return nil, nil
	}

	for _, nw := range m.Machine.Allocation.Networks {
		if nw == nil {
			continue
		}
		networkID := *nw.Networkid
		nwResp, err := r.resources.client.NetworkGet(networkID)
		if err != nil {
			return nil, nil
		}
		if nwResp == nil || nwResp.Network == nil || nwResp.Network.Parentnetworkid == nil {
			continue
		}

		resp, err := r.resources.client.FirewallList()
		if err != nil {
			return nil, err
		}

		for _, fw := range resp.Firewalls {
			if fw == nil || fw.Allocation == nil {
				continue
			}

			for _, nw = range fw.Allocation.Networks {
				if nw == nil || nw.Networkid == nil || *nw.Networkid != networkID {
					continue
				}

				return fw, nil
			}
		}
	}

	return nil, nil
}

// SyncMetalLBConfig synchronizes the Metal-LB config.
func (r *ResourcesController) SyncMetalLBConfig(nodes []*v1.Node) error {
	for _, node := range nodes {
		resp, err := machineByHostname(r.resources.client, types.NodeName(node.Name))
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		machine := resp.Machine
		r.metallbConfig.announceMachineIPs(machine)

		podCIDR := node.Spec.PodCIDR
		peer, err := r.metallbConfig.getPeer(podCIDR)
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		if peer != nil {
			continue
		}

		peer, err = NewPeer(machine, podCIDR)
		if err != nil {
			runtime.HandleError(err)
			continue
		}
		r.metallbConfig.Peers = append(r.metallbConfig.Peers, peer)
	}

	return r.upsertMetalLBConfig()
}

// upsertMetalLBConfig inserts or updates Metal-LB config.
func (r *ResourcesController) upsertMetalLBConfig() error {
	yaml, err := r.metallbConfig.ToYAML()
	if err != nil {
		return nil
	}

	cm := make(map[string]string, 1)
	cm[configMapKey] = yaml

	return r.upsertConfigMap(namespace, configMapName, cm)
}
