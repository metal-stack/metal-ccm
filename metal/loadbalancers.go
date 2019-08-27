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

//nolint
package metal

import (
	"context"
	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	"k8s.io/cloud-provider"
	"k8s.io/component-base/logs"
	"strings"

	"errors"
	"fmt"
	"log"
)

const (
	// loadBalancerIDAnnotation is the annotation specifying the load-balancer ID
	// used to enable fast retrievals of load-balancers from the API by UUID.
	loadBalancerIDAnnotation = "kubernetes.metal.com/load-balancer-id"
)

var errLBNotFound = errors.New("loadBalancerController not found")

type loadBalancerController struct {
	resources *resources
	logger    *log.Logger
	resctl    *ResourcesController
	lbs       []*loadBalancer
}

func newLoadBalancerController(resources *resources) *loadBalancerController {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm loadbalancers | ")

	return &loadBalancerController{
		resources: resources,
		logger:    logger,
	}
}

type loadBalancer struct {
	name string
	id   string
	ip   string
}

func newLoadBalancer(name, id, ip string) *loadBalancer {
	return &loadBalancer{
		name: name,
		id:   id,
		ip:   ip,
	}
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (lbc *loadBalancerController) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lbc.logger.Printf("GetLoadBalancer: ClusterName %q, Namespace %q, ServiceName %q\n", clusterName, service.Namespace, service.Name)
	lb, err := lbc.retrieveAndAnnotateLoadBalancer(ctx, service)
	if err != nil {
		if err == errLBNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.ip,
			},
		},
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer.
func (lbc *loadBalancerController) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	lbc.logger.Printf("GetLoadBalancerName: ClusterName %q, Namespace %q, ServiceName %q\n", clusterName, service.Namespace, service.Name)
	return lbc.lbName(service)
}

func (lbc *loadBalancerController) lbName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

func nodeNames(nodes []*v1.Node) string {
	var nn []string
	for _, n := range nodes {
		nn = append(nn, n.Name)
	}
	return strings.Join(nn, ",")
}

// EnsureLoadBalancer ensures that the cluster is running a load balancer for service.
// It creates a new load balancer or updates the existing one. Returns the status of the balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (lbc *loadBalancerController) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbc.logger.Printf("EnsureLoadBalancer: ClusterName %q, Namespace %q, ServiceName %q, Nodes:\n%s\n", clusterName, service.Namespace, service.Name, nodeNames(nodes))

	name := lbc.lbName(service)
	id := uuid.New().String()
	ip := "10.100.0.1" //TODO
	lb := newLoadBalancer(name, id, ip)

	lbc.lbs = append(lbc.lbs, lb)

	//TODO lbc.restctl.AcquireIPs(project, network, count)

	err := lbc.resctl.SyncMetalLBConfig()
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.ip,
			},
		},
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (lbc *loadBalancerController) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lbc.logger.Printf("UpdateLoadBalancer: ClusterName %q, Namespace %q, ServiceName %q, Nodes:\n%s\n", clusterName, service.Namespace, service.Name, nodeNames(nodes))

	err := lbc.resctl.SyncMetalLBConfig()
	if err != nil {
		return err
	}

	return nil
}

// EnsureLoadBalancerDeleted deletes the cluster load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Parameter 'service' is not modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (lbc *loadBalancerController) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lbc.logger.Printf("EnsureLoadBalancerDeleted: ClusterName %q, Namespace %q, ServiceName %q\n", clusterName, service.Namespace, service.Name)

	existingLB, err := lbc.retrieveLoadBalancer(ctx, service)
	if err != nil {
		if err == errLBNotFound {
			return nil
		}
		return err
	}

	for i, lb := range lbc.lbs {
		if lb.id == existingLB.id {
			lbc.lbs = append(lbc.lbs[:i], lbc.lbs[i+1:]...)
			break
		}
	}

	return nil
}

func (lbc *loadBalancerController) retrieveAndAnnotateLoadBalancer(ctx context.Context, service *v1.Service) (*loadBalancer, error) {
	lb, err := lbc.retrieveLoadBalancer(ctx, service)
	if err != nil {
		return nil, err
	}

	if err := lbc.ensureLoadBalancerIDAnnotation(service, lb.id); err != nil {
		return nil, fmt.Errorf("failed to add load-balancer ID annotation to service %s/%s: %s", service.Namespace, service.Name, err)
	}

	return lb, nil
}

func (lbc *loadBalancerController) retrieveLoadBalancer(ctx context.Context, service *v1.Service) (*loadBalancer, error) {
	id := getLoadBalancerID(service)
	if len(id) > 0 {
		for _, lb := range lbc.lbs {
			if lb.id == id {
				return lb, nil
			}
		}
	}

	return nil, errLBNotFound
}

func (lbc *loadBalancerController) ensureLoadBalancerIDAnnotation(service *v1.Service, lbID string) error {
	if val := getLoadBalancerID(service); val == lbID {
		return nil
	}

	updated := service.DeepCopy()
	if updated.ObjectMeta.Annotations == nil {
		updated.ObjectMeta.Annotations = map[string]string{}
	}
	updated.ObjectMeta.Annotations[loadBalancerIDAnnotation] = lbID

	return patchService(lbc.resources.kclient, service, updated)
}

func getLoadBalancerID(service *v1.Service) string {
	return service.ObjectMeta.Annotations[loadBalancerIDAnnotation]
}
