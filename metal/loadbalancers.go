package metal

import (
	"context"
	"github.com/google/uuid"
	"k8s.io/api/core/v1"
	"k8s.io/cloud-provider"
	"k8s.io/component-base/logs"
	"strconv"
	"strings"

	"errors"
	"fmt"
	"log"
)

const (
	projectIDAnnotation = "project-id"
	networkIDAnnotation = "network-id"
	ipCountAnnotation   = "ip-count"
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
	lbc.logger.Printf("GetLoadBalancer: clusterName %q, namespace %q, serviceName %q\n", clusterName, service.Namespace, service.Name)
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
	lbc.logger.Printf("GetLoadBalancerName: clusterName %q, namespace %q, serviceName %q\n", clusterName, service.Namespace, service.Name)
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
	lbc.logger.Printf("EnsureLoadBalancer: clusterName %q, namespace %q, serviceName %q, nodes %q\n", clusterName, service.Namespace, service.Name, nodeNames(nodes))
	err := lbc.acquireIPs(service)
	if err != nil {
		return nil, err
	}

	err = lbc.resctl.SyncMetalLBConfig()
	if err != nil {
		return nil, err
	}

	id := uuid.New().String()
	lb := newLoadBalancer(service.Name, id, service.Spec.LoadBalancerIP)
	lbc.lbs = append(lbc.lbs, lb)

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
	lbc.logger.Printf("UpdateLoadBalancer: clusterName %q, namespace %q, serviceName %q, nodes %q\n", clusterName, service.Namespace, service.Name, nodeNames(nodes))
	err := lbc.acquireIPs(service)
	if err != nil {
		return err
	}

	err = lbc.resctl.SyncMetalLBConfig()
	if err != nil {
		return err
	}

	return nil
}

func (lbc *loadBalancerController) acquireIPs(service *v1.Service) error {
	projectID := service.Annotations[projectIDAnnotation]
	if len(projectID) == 0 {
		//Will prevent services with no 'project-id' annotation from being loadbalanced
		//return fmt.Errorf("service %q does not have %q annotation", service.Name, projectIDAnnotation)
		return nil
	}

	networkID := service.Annotations[networkIDAnnotation]
	if len(networkID) == 0 {
		//Will prevent services with no 'network-id' annotation from being loadbalanced
		//return fmt.Errorf("service %q does not have %q annotation", service.Name, networkIDAnnotation)
		return nil
	}

	ipCount := service.Annotations[ipCountAnnotation]
	if len(ipCount) == 0 {
		//Will prevent services with no 'ip-count' annotation from being loadbalanced
		//return fmt.Errorf("service %q does not have %q annotation", service.Name, ipCountAnnotation)
		return nil
	}

	count, err := strconv.Atoi(ipCount)
	if err != nil {
		return fmt.Errorf("service %q has invalid %q annotation: integer expected", service.Name, ipCountAnnotation)
	}
	if count < 1 {
		return fmt.Errorf("service %q has invalid %q annotation: positive integer expected", service.Name, ipCountAnnotation)
	}

	return lbc.resctl.AcquireIPs(projectID, networkID, count)
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
	lbc.logger.Printf("EnsureLoadBalancerDeleted: clusterName %q, namespace %q, serviceName %q\n", clusterName, service.Namespace, service.Name)

	existingLB, err := lbc.retrieveLoadBalancer(ctx, service)
	if err != nil {
		if err == errLBNotFound {
			return nil
		}
		return err
	}

	for i, lb := range lbc.lbs {
		if lb.id == existingLB.id {
			err := lbc.resctl.DeleteIPs() //TODO delete network IPs
			if err != nil {
				return err
			}

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
