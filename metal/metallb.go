package metal

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	prefix        = "metallb-"
	namespace     = "metallb-system"
	configMapName = "config"
	configMapKey  = "config"
)

// AddFirewallNetworkAddressPools creates and adds empty address pools for all non-private and non-underlay firewall networks.
func (r *ResourcesController) AddFirewallNetworkAddressPools() error {
	resp, err := r.resources.client.FirewallList()
	if err != nil {
		return err
	}

	var networkIDs []string
	for _, fw := range resp.Firewalls {
		if fw == nil || fw.Allocation == nil {
			continue
		}

		for _, nw := range fw.Allocation.Networks {
			if nw == nil || (nw.Private != nil && *nw.Private) || (nw.Underlay != nil && *nw.Underlay) || nw.Networkid == nil || len(*nw.Networkid) == 0 {
				continue
			}

			existent := false
			for _, id := range networkIDs {
				if id == *nw.Networkid {
					existent = true
					break
				}
			}

			if !existent {
				networkIDs = append(networkIDs, *nw.Networkid)
				r.metallbConfig.AddressPools = append(r.metallbConfig.AddressPools, NewBGPAddressPool(*nw.Networkid))
			}
		}
	}

	return r.upsertMetalLBConfig()
}

// SyncMetalLBConfig synchronizes the Metal-LB config.
func (r *ResourcesController) SyncMetalLBConfig() error {
	nodes, err := r.getNodes()
	if err != nil {
		return err
	}

	for _, node := range nodes {
		nodeName := node.GetName()
		resp, err := machineByHostname(r.resources.client, types.NodeName(nodeName))
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
