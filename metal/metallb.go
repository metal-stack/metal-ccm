package metal

import (
	metalgo "github.com/metal-pod/metal-go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"strings"
)

const (
	namespace     = "metallb-system"
	configMapName = "config"
	configMapKey  = "config"
)

// SyncMetalLBConfig synchronizes the Metal-LB config.
func (r *ResourcesController) SyncMetalLBConfig(nodes []*v1.Node, project, network string) error {
	for _, node := range nodes {
		resp, err := machineByHostname(r.resources.client, types.NodeName(node.Name))
		if err != nil {
			runtime.HandleError(err)
			continue
		}

		machine := resp.Machine
		err = r.addExistentIPs(project, network)
		if err != nil {
			runtime.HandleError(err)
			continue
		}

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

// addExistentIPs appends the already existent external network IPs of the given network within the given project to their corresponding address pools.
func (r *ResourcesController) addExistentIPs(project, network string) error {
	req := &metalgo.IPFindRequest{
		ProjectID: &project,
		NetworkID: &network,
	}
	resp, err := r.resources.client.IPFind(req)
	if err != nil {
		return err
	}

	var ips []string
	for _, ip := range resp.IPs {
		if strings.Contains(ip.Name, prefix) {
			ips = append(ips, *ip.Ipaddress)
		}
	}

	r.metallbConfig.announceIPs(network, ips)
	return nil
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
