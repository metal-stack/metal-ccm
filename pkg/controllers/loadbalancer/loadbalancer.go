package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/metal-pod/metal-ccm/pkg/resources/constants"
	"github.com/metal-pod/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-pod/metal-ccm/pkg/resources/metal"

	metalgo "github.com/metal-pod/metal-go"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/component-base/logs"
)

const (
	// loadBalancerIDAnnotation is the annotation specifying the load-balancer ID
	// used to enable fast retrievals of load-balancers from the API by UUID.
	loadBalancerIDAnnotation = "kubernetes.metal.com/load-balancer-id"
)

var errLBNotFound = errors.New("loadBalancerController not found")

type LoadBalancerController struct {
	client      *metalgo.Driver
	partitionID string
	projectID   string
	logger      *log.Logger
	K8sClient   clientset.Interface
	mtx         *sync.Mutex
}

// New returns a new load balancer controller that satisfies the kubernetes cloud provider load balancer interface
func New(client *metalgo.Driver, partitionID, projectID string) *LoadBalancerController {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm loadbalancer | ")

	return &LoadBalancerController{
		client:      client,
		logger:      logger,
		partitionID: partitionID,
		projectID:   projectID,
		mtx:         &sync.Mutex{},
	}
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l *LoadBalancerController) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	l.logger.Printf("GetLoadBalancer: clusterName %q, namespace %q, serviceName %q", clusterName, service.Namespace, service.Name)

	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return nil, false, nil
	}

	return &v1.LoadBalancerStatus{
		Ingress: service.Status.LoadBalancer.Ingress,
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer.
func (l *LoadBalancerController) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	l.logger.Printf("GetLoadBalancerName: clusterName %q, namespace %q, serviceName %q\n", clusterName, service.Namespace, service.Name)

	return l.lbName(service)
}

func (l *LoadBalancerController) lbName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

// EnsureLoadBalancer ensures that the cluster is running a load balancer for service.
// It creates a new load balancer or updates the existing one. Returns the status of the balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l *LoadBalancerController) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	l.logger.Printf("EnsureLoadBalancer: clusterName %q, namespace %q, serviceName %q, nodes %q", clusterName, service.Namespace, service.Name, kubernetes.NodeNamesOfNodes(nodes))

	if len(nodes) == 0 {
		return nil, fmt.Errorf("EnsureLoadBalancer: unable to satisfy load balancer request as there is no node yet in the cluster")
	}

	var currentIPs []string
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		currentIPs = append(currentIPs, ingress.IP)
	}

	currentIPCount := len(currentIPs)
	wantedIPCount, err := extractIPCountAnnotation(service)
	if err != nil {
		return nil, err
	}

	l.mtx.Lock()
	defer l.mtx.Unlock()

	var newIPs []string
	if currentIPCount < wantedIPCount {
		newIPs, err = l.acquireIPs(service, wantedIPCount-currentIPCount)
		if err != nil {
			return nil, err
		}
	}

	// TODO: For now we just add all IPs of this project to the metallb config
	// this will become more controllable when announceable ips are implemented in network entities of the metal-api
	ips, err := metal.FindProjectIPs(l.client, l.projectID)
	if err != nil {
		return nil, fmt.Errorf("could not find ips of this project: %v", err)
	}
	networks, err := metal.ListNetworks(l.client)
	if err != nil {
		return nil, fmt.Errorf("could not list networks: %v", err)
	}
	networkMap := metal.NetworksByID(networks)

	config := newMetalLBConfig()
	for _, ip := range ips {
		nw, ok := networkMap[*ip.Networkid]
		if !ok {
			continue
		}
		// we do not want IPs in networks where the parents are private or underlay networks
		if nw.Parentnetworkid != nil && *nw.Parentnetworkid != "" {
			parent, ok := networkMap[*nw.Parentnetworkid]
			if !ok {
				continue
			}
			if *parent.Privatesuper || *parent.Underlay {
				continue
			}
		}
		config.addIPToPool(*ip.Networkid, *ip.Ipaddress)
	}
	config.Write(l.K8sClient)

	lbStatusIngress := []v1.LoadBalancerIngress{}
	for _, ip := range newIPs {
		lbStatusIngress = append(lbStatusIngress, v1.LoadBalancerIngress{IP: ip})
	}

	return &v1.LoadBalancerStatus{
		Ingress: lbStatusIngress,
	}, nil
}

func extractIPCountAnnotation(service *v1.Service) (int, error) {
	annotations := service.GetAnnotations()

	ipCountString, ok := annotations[constants.IPCountServiceAnnotation]
	if !ok {
		return 1, nil
	}

	count, err := strconv.Atoi(ipCountString)
	if err != nil {
		return 0, fmt.Errorf("service %q has invalid %q label: integer expected", service.Name, ipCountString)
	}
	if count < 1 {
		return 0, fmt.Errorf("service %q has invalid %q label: positive integer expected", service.Name, ipCountString)
	}

	return count, nil
}

func (l *LoadBalancerController) acquireIPs(service *v1.Service, ipCount int) ([]string, error) {
	if len(service.Spec.ExternalIPs) == 0 {
		return l.acquireIPsFromDefaultExternalNetwork(ipCount)
	}

	// TODO: Implement acquiring from explicit external networks
	return nil, errors.New("not implemented")
}

func (l *LoadBalancerController) acquireIPsFromDefaultExternalNetwork(ipCount int) ([]string, error) {
	nwID, err := l.getExternalNetworkID()
	if err != nil {
		return nil, err
	}

	ips, err := metal.AcquireIPs(l.client, constants.IPPrefix, l.projectID, nwID, ipCount)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire IPs for project %q in network %q: %v", l.projectID, nwID, err)
	}
	ipStrings := metal.IPAddressesOfIPs(ips)
	l.logger.Printf("acquired ips in external network %q: %v", nwID, ipStrings)

	return ipStrings, nil
}

func (l *LoadBalancerController) getExternalNetworkID() (string, error) {
	externalNWs, err := metal.FindExternalNetworksInPartition(l.client, l.partitionID)
	if err != nil {
		return "", err
	}

	if len(externalNWs) == 0 {
		return "", fmt.Errorf("no external network(s) found: %v", err)
	}

	// we use the network that starts with "internet" as the default external network... this is a convention
	// it would be nicer if there was a field in the network entity though
	for _, enw := range externalNWs {
		if strings.HasPrefix(*enw.ID, "internet") {
			return *enw.ID, nil
		}
	}

	return "", errors.New("no default external network(s) found")
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l *LoadBalancerController) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	l.logger.Printf("UpdateLoadBalancer: clusterName %q, namespace %q, serviceName %q, nodes %q\n", clusterName, service.Namespace, service.Name, kubernetes.NodeNamesOfNodes(nodes))

	l.mtx.Lock()
	defer l.mtx.Unlock()

	return fmt.Errorf("updating load balancer not yet supported")
}

// EnsureLoadBalancerDeleted deletes the cluster load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Parameter 'service' is not modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *LoadBalancerController) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	l.logger.Printf("EnsureLoadBalancerDeleted: clusterName %q, namespace %q, serviceName %q\n", clusterName, service.Namespace, service.Name)

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		err := metal.DeleteIP(l.client, ingress.IP)
		if err != nil {
			l.logger.Printf("unable to delete ip: %s", ingress.IP)
		}
	}

	return nil
}
