package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"log"
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

type LoadBalancerController struct {
	client           *metalgo.Driver
	partitionID      string
	projectID        string
	networkID        string
	logger           *log.Logger
	K8sClient        clientset.Interface
	configWriteMutex *sync.Mutex
	ipAllocateMutex  *sync.Mutex
}

// New returns a new load balancer controller that satisfies the kubernetes cloud provider load balancer interface
func New(client *metalgo.Driver, partitionID, projectID, networkID string) *LoadBalancerController {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm loadbalancer | ")

	return &LoadBalancerController{
		client:           client,
		logger:           logger,
		partitionID:      partitionID,
		projectID:        projectID,
		networkID:        networkID,
		configWriteMutex: &sync.Mutex{},
		ipAllocateMutex:  &sync.Mutex{},
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

	ingressStatus := service.Status.LoadBalancer.Ingress

	// FIXME implement based on our findings/decisions.
	fixedIP := service.Spec.LoadBalancerIP
	if fixedIP != "" {
		l.logger.Printf("fixed ip:%s requested, ignoring for now.", fixedIP)
		return &v1.LoadBalancerStatus{Ingress: ingressStatus}, nil
	}

	l.ipAllocateMutex.Lock()
	defer l.ipAllocateMutex.Unlock()

	// if we already acquired an IP, we write it into the service status
	// we do not acquire another IP if there is already an IP present in the service status
	currentIPCount := len(ingressStatus)
	var err error
	var ip string
	if currentIPCount < 1 {
		ip, err = l.acquireIP(service)
		if err != nil {
			return nil, err
		}
		ingressStatus = append(ingressStatus, v1.LoadBalancerIngress{IP: ip})
	}

	err = l.UpdateMetalLBConfig(nodes)
	if err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: ingressStatus,
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l *LoadBalancerController) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	return l.UpdateMetalLBConfig(nodes)
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

	nodes, err := kubernetes.GetNodes(l.K8sClient)
	if err != nil {
		return err
	}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		err := metal.DeleteIP(l.client, ingress.IP)
		if err != nil {
			return fmt.Errorf("unable to delete ip: %s", ingress.IP)
		}
	}

	return l.UpdateMetalLBConfig(nodes)
}

// UpdateMetalLBConfig the metallb config for given nodes
func (l *LoadBalancerController) UpdateMetalLBConfig(nodes []*v1.Node) error {
	l.configWriteMutex.Lock()
	defer l.configWriteMutex.Unlock()

	err := l.updateLoadBalancerConfig(nodes)
	if err != nil {
		return err
	}

	l.logger.Printf("metallb config updated successfully")

	return nil
}

func (l *LoadBalancerController) acquireIP(service *v1.Service) (string, error) {
	annotations := service.GetAnnotations()
	addressPool, ok := annotations[constants.MetalLBSpecificAddressPool]
	if !ok {
		return l.acquireIPFromDefaultExternalNetwork()
	}
	return l.acquireIPFromSpecificNetwork(addressPool)
}

func (l *LoadBalancerController) acquireIPFromDefaultExternalNetwork() (string, error) {
	nwID, err := l.getExternalNetworkID()
	if err != nil {
		return "", err
	}

	return l.acquireIPFromSpecificNetwork(nwID)
}

func (l *LoadBalancerController) acquireIPFromSpecificNetwork(nwID string) (string, error) {
	ip, err := metal.AcquireIP(l.client, constants.IPPrefix, l.projectID, nwID)
	if err != nil {
		return "", fmt.Errorf("failed to acquire IPs for project %q in network %q: %v", l.projectID, nwID, err)
	}

	l.logger.Printf("acquired ip in network %q: %v", nwID, *ip.Ipaddress)

	return *ip.Ipaddress, nil
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

func (l *LoadBalancerController) updateLoadBalancerConfig(nodes []*v1.Node) error {
	// TODO: For now we just add all IPs of this project to the metallb config
	// this will become more controllable when announceable ips are implemented in network entities of the metal-api
	ips, err := metal.FindProjectIPs(l.client, l.projectID)
	if err != nil {
		return fmt.Errorf("could not find ips of this project: %v", err)
	}
	networks, err := metal.ListNetworks(l.client)
	if err != nil {
		return fmt.Errorf("could not list networks: %v", err)
	}
	networkMap := metal.NetworksByID(networks)

	config := newMetalLBConfig()
	err = config.CalculateConfig(ips, networkMap, nodes)
	if err != nil {
		return err
	}
	err = config.Write(l.K8sClient)
	if err != nil {
		return err
	}
	return nil
}
