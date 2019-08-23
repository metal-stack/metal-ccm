package metal

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	prefix        = "metallb-"
	namespace     = "metallb-system"
	configMapName = "config"
)

// AddFirewallNetworkAddressPools creates and adds empty address pools for all non-private and non-underlay firewall networks.
func (r *ResourcesController) AddFirewallNetworkAddressPools() error {
	resp, err := r.resources.client.FirewallList()
	if err != nil {
		return err
	}

	for _, fw := range resp.Firewalls {
		if fw == nil || fw.Allocation == nil {
			continue
		}

		for _, nw := range fw.Allocation.Networks {
			if *nw.Private || *nw.Underlay {
				continue
			}

			r.metalLBConfig.AddressPools = append(r.metalLBConfig.AddressPools, NewBGPAddressPool(*nw.Networkid))
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
		r.metalLBConfig.announceMachineIPs(machine)

		podCIDR := node.Spec.PodCIDR
		peer, err := r.metalLBConfig.getPeer(podCIDR)
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
		r.metalLBConfig.Peers = append(r.metalLBConfig.Peers, peer)
	}

	return r.upsertMetalLBConfig()
}

// upsertMetalLBConfig inserts or updates Metal-LB config.
func (r *ResourcesController) upsertMetalLBConfig() error {
	var configMap map[string]string
	marshalledConfig, err := json.Marshal(r.metalLBConfig)
	if err != nil {
		return err
	}

	err = json.Unmarshal(marshalledConfig, &configMap)
	if err != nil {
		return err
	}

	return r.upsertConfigMap(namespace, configMapName, configMap)
}
