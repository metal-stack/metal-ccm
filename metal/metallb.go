package metal

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const (
	namespace     = "metallb-system"
	configMapName = "config"
	configMapKey  = "config"
)

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
