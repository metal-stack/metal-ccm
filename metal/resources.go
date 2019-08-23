/*
Copyright 2017 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metal

import (
	"fmt"
	"k8s.io/api/core/v1"
	"strings"
	"time"

	"github.com/metal-pod/metal-go"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
)

const (
	controllerSyncTagsPeriod = 1 * time.Minute
)

var (
	updateNodeSpecBackoff = wait.Backoff{
		Steps:    20,
		Duration: 50 * time.Millisecond,
		Jitter:   1.0,
	}
)

type resources struct {
	client    *metalgo.Driver
	kclient   kubernetes.Interface
	instances *instances
}

// newResources initializes a new resources instance.

// kclient can only be set during the cloud.Initialize call since that is when
// the cloud provider framework provides us with a clientset. Fortunately, the
// initialization order guarantees that kclient won't be consumed prior to it
// being set.
func newResources(client *metalgo.Driver, i *instances) *resources {
	return &resources{
		client:    client,
		instances: i,
	}
}

type syncer interface {
	Sync(name string, period time.Duration, stopCh <-chan struct{}, fn func() error)
}

type tickerSyncer struct{}

func (s *tickerSyncer) Sync(name string, period time.Duration, stopCh <-chan struct{}, fn func() error) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	// manually call to avoid initial tick delay
	if err := fn(); err != nil {
		klog.Errorf("%s failed: %s", name, err)
	}

	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				klog.Errorf("%s failed: %s", name, err)
			}
		case <-stopCh:
			return
		}
	}
}

// ResourcesController is responsible for managing metal cloud
// resources. It maintains a local state of the resources and
// synchronizes when needed.
type ResourcesController struct {
	kclient    kubernetes.Interface
	nodeLister v1lister.NodeLister

	resources *resources
	syncer    syncer

	metalLBConfig *MetalLBConfig
}

// NewResourcesController returns a new resource controller.
func NewResourcesController(r *resources, inf v1informer.NodeInformer, client kubernetes.Interface) *ResourcesController {
	r.kclient = client
	return &ResourcesController{
		resources:     r,
		kclient:       client,
		nodeLister:    inf.Lister(),
		syncer:        &tickerSyncer{},
		metalLBConfig: &MetalLBConfig{},
	}
}

// Run starts the resources controller loop.
func (r *ResourcesController) Run(stopCh <-chan struct{}) {
	r.syncer.Sync("tags syncer", controllerSyncTagsPeriod, stopCh, r.syncMachineTagsToNodeLabels)
}

// getMachineTags returns all machine tags within the shoot.
func (r *ResourcesController) getMachineTags(nodes []*v1.Node) (map[string][]string, error) {
	machines := r.resources.instances.getMachines(nodes)
	machineTags := make(map[string][]string)
	for _, m := range machines {
		hostname := *m.Allocation.Hostname
		machineTags[hostname] = m.Tags
		klog.Infof("machine %s ", hostname)
	}
	return machineTags, nil
}

// getNodes returns all nodes of this cluster.
func (r *ResourcesController) getNodes() ([]*v1.Node, error) {
	nodes, err := r.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %s", err)
	}
	return nodes, nil
}

// syncMachineTagsToNodeLabels synchronizes tags of machines in this project to labels of that node.
func (r *ResourcesController) syncMachineTagsToNodeLabels() error {
	nodes, err := r.getNodes()
	if err != nil {
		return err
	}

	machineTags, err := r.getMachineTags(nodes)
	if err != nil {
		return err
	}

	// klog.Infof("nodes: %s", nodes)
	for _, n := range nodes {
		nodeName := n.GetName()
		tags, ok := machineTags[nodeName]
		if !ok {
			klog.Warningf("node:%s not a machine", nodeName)
			continue
		}
		ll := buildLabels(tags)

		for key, value := range ll {
			klog.Infof("machine label: %s value:%s", key, value)
			_, ok := n.Labels[key]
			if ok {
				klog.Infof("skip existing node label:%s", key)
				continue
			}
			klog.Infof("adding node label from metal: %s=%s to node:%s", key, value, nodeName)
			n.Labels[key] = value
		}
	}

	r.updateNodes(nodes)

	return nil
}

// updateNodes updates given nodes.
func (r *ResourcesController) updateNodes(nodes []*v1.Node) {
	for _, n := range nodes {
		err := retry.RetryOnConflict(updateNodeSpecBackoff, func() error {
			_, err := r.kclient.CoreV1().Nodes().Update(n)
			r.kclient.CoreV1().ConfigMaps("")
			return err
		})
		if err != nil {
			runtime.HandleError(err)
		}
	}
}

func buildLabels(tags []string) map[string]string {
	result := make(map[string]string)
	for _, t := range tags {
		parts := strings.Split(t, "=")
		// TODO labels must have a value ?
		// if len(parts) == 0 {
		// 	result[t] = ""
		// }
		// if len(parts) == 1 {
		// 	result[parts[0]] = ""
		// }
		if len(parts) >= 2 {
			result[parts[0]] = strings.Join(parts[1:], "=")
		}
	}
	return result
}
