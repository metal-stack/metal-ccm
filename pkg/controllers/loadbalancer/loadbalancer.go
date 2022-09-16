package loadbalancer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/metal-stack/metal-ccm/pkg/tags"
	"github.com/metal-stack/metal-lib/pkg/tag"

	"github.com/metal-stack/metal-ccm/pkg/resources/constants"
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-stack/metal-ccm/pkg/resources/metal"
	metalip "github.com/metal-stack/metal-go/api/client/ip"
	"github.com/metal-stack/metal-go/api/models"

	metalgo "github.com/metal-stack/metal-go"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	retrygo "github.com/avast/retry-go/v4"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	cloudprovider "k8s.io/cloud-provider"
)

type LoadBalancerController struct {
	client                   metalgo.Client
	partitionID              string
	projectID                string
	clusterID                string
	defaultExternalNetworkID string
	additionalNetworks       sets.String
	K8sClient                clientset.Interface
	configWriteMutex         *sync.Mutex
	ipAllocateMutex          *sync.Mutex
	ipUpdateMutex            *sync.Mutex
}

// New returns a new load balancer controller that satisfies the kubernetes cloud provider load balancer interface
func New(client metalgo.Client, partitionID, projectID, clusterID, defaultExternalNetworkID string, additionalNetworks []string) *LoadBalancerController {
	return &LoadBalancerController{
		client:                   client,
		partitionID:              partitionID,
		projectID:                projectID,
		clusterID:                clusterID,
		defaultExternalNetworkID: defaultExternalNetworkID,
		additionalNetworks:       sets.NewString(additionalNetworks...),
		configWriteMutex:         &sync.Mutex{},
		ipAllocateMutex:          &sync.Mutex{},
		ipUpdateMutex:            &sync.Mutex{},
	}
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l *LoadBalancerController) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	klog.Infof("GetLoadBalancer: clusterName %q, namespace %q, serviceName %q", clusterName, service.Namespace, service.Name)

	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return nil, false, nil
	}

	return &v1.LoadBalancerStatus{
		Ingress: service.Status.LoadBalancer.Ingress,
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer.
func (l *LoadBalancerController) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	klog.Infof("GetLoadBalancerName: clusterName %q, namespace %q, serviceName %q\n", clusterName, service.Namespace, service.Name)

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
	ns := []v1.Node{}
	for i := range nodes {
		ns = append(ns, *nodes[i])
	}
	klog.Infof("EnsureLoadBalancer: clusterName %q, namespace %q, serviceName %q, nodes %q", clusterName, service.Namespace, service.Name, kubernetes.NodeNamesOfNodes(ns))

	ingressStatus := service.Status.LoadBalancer.Ingress

	fixedIP := service.Spec.LoadBalancerIP
	if fixedIP != "" {
		l.ipUpdateMutex.Lock()
		defer l.ipUpdateMutex.Unlock()

		ip, err := metal.FindProjectIP(l.client, l.projectID, fixedIP)
		if err != nil {
			return nil, err
		}
		newIP, err := l.useIPInCluster(*ip, l.clusterID, *service)
		if err != nil {
			klog.Errorf("could not associate fixed ip:%s, err: %v", fixedIP, err)
			return nil, err
		}
		ingressStatus = append(ingressStatus, v1.LoadBalancerIngress{IP: *newIP.Ipaddress})
		return &v1.LoadBalancerStatus{Ingress: ingressStatus}, nil
	}

	l.ipAllocateMutex.Lock()
	defer l.ipAllocateMutex.Unlock()

	// if we already acquired an IP, we write it into the service status
	// we do not acquire another IP if there is already an IP present in the service status
	currentIPCount := len(ingressStatus)
	var err error
	var ip string
	if currentIPCount > 0 {
		return &v1.LoadBalancerStatus{
			Ingress: ingressStatus,
		}, nil
	}

	ip, err = l.acquireIP(service)
	if err != nil {
		return nil, err
	}

	rollback := func(err error) error {
		if err == nil {
			return nil
		}

		klog.Errorf("error while trying to ensure load balancer, rolling back ip acquisition: %v", err)

		// clearing tags before release
		// we can do this because here we know that we freshly acquired a new IP that's not used for anything else
		_, err2 := l.client.IP().UpdateIP(metalip.NewUpdateIPParams().WithBody(&models.V1IPUpdateRequest{
			Ipaddress: &ip,
			Tags:      []string{},
		}), nil)
		if err != nil {
			klog.Errorf("error during ip rollback occurred: %v", err2)
			return err
		}

		_, err2 = l.client.IP().FreeIP(metalip.NewFreeIPParams().WithID(ip), nil)
		if err2 != nil {
			klog.Errorf("error during ip rollback occurred: %v", err2)
			return err
		}

		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := l.K8sClient.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		s.Spec.LoadBalancerIP = ip
		_, err = l.K8sClient.CoreV1().Services(s.Namespace).Update(ctx, s, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, rollback(err)
	}

	ingressStatus = append(ingressStatus, v1.LoadBalancerIngress{IP: ip})

	err = l.UpdateMetalLBConfig(ns)
	if err != nil {
		return nil, rollback(err)
	}

	return &v1.LoadBalancerStatus{
		Ingress: ingressStatus,
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l *LoadBalancerController) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	ns := []v1.Node{}
	for i := range nodes {
		ns = append(ns, *nodes[i])
	}
	return l.UpdateMetalLBConfig(ns)
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
	klog.Infof("EnsureLoadBalancerDeleted: clusterName %q, namespace %q, serviceName %q, serviceStatus: %v\n", clusterName, service.Namespace, service.Name, service.Status)

	s := *service
	serviceTag := tags.BuildClusterServiceFQNTag(l.clusterID, s.GetNamespace(), s.GetName())
	ips, err := metal.FindProjectIPsWithTag(l.client, l.projectID, serviceTag)
	if err != nil {
		return err
	}

	l.ipUpdateMutex.Lock()
	defer l.ipUpdateMutex.Unlock()

	for _, ip := range ips {
		ip := ip
		err := retrygo.Do(
			func() error {
				newTags, last := l.removeServiceTag(*ip, serviceTag)
				iu := &models.V1IPUpdateRequest{
					Ipaddress: ip.Ipaddress,
					Tags:      newTags,
				}
				newIP, err := l.client.IP().UpdateIP(metalip.NewUpdateIPParams().WithBody(iu), nil)
				if err != nil {
					return fmt.Errorf("could not update ip with new tags: %w", err)
				}
				klog.Infof("updated ip: %v", newIP)
				if *ip.Type == models.V1IPBaseTypeEphemeral && last {
					klog.Infof("freeing unused ephemeral ip: %s, tags: %s", *ip.Ipaddress, newTags)
					err := metal.FreeIP(l.client, *ip.Ipaddress)
					if err != nil {
						return fmt.Errorf("unable to delete ip %s: %w", *ip.Ipaddress, err)
					}
				}
				return nil
			},
		)
		if err != nil {
			return err
		}
	}

	// we do not update the metallb config here because then the metallb controller will report a stale config
	// this is because the service gets deleted after updating the metallb config map
	//
	// therefore, we let the housekeeping update the config map

	return nil
}

// removes the service tag and checks whether it is the last service tag.
func (l *LoadBalancerController) removeServiceTag(ip models.V1IPResponse, serviceTag string) ([]string, bool) {
	count := 0
	newTags := []string{}
	for _, t := range ip.Tags {
		if strings.HasPrefix(t, tag.ClusterServiceFQN) {
			count++
		}
		if t == serviceTag {
			continue
		}
		newTags = append(newTags, t)
	}
	last := (count <= 1)
	klog.Infof("removing service tag '%s', last: %t, oldTags: %v, newTags: %v", serviceTag, last, ip.Tags, newTags)
	return newTags, last
}

// UpdateMetalLBConfig the metallb config for given nodes
func (l *LoadBalancerController) UpdateMetalLBConfig(nodes []v1.Node) error {
	l.configWriteMutex.Lock()
	defer l.configWriteMutex.Unlock()

	err := l.updateLoadBalancerConfig(nodes)
	if err != nil {
		return err
	}

	klog.Info("metallb config updated successfully")

	return nil
}

func (l *LoadBalancerController) useIPInCluster(ip models.V1IPResponse, clusterID string, s v1.Service) (*models.V1IPResponse, error) {
	for _, t := range ip.Tags {
		if tags.IsMachine(t) {
			return nil, fmt.Errorf("ip is used for a machine, can not use it for a service, ip tags: %v", ip.Tags)
		}
		if tags.IsEgress(t) {
			return nil, fmt.Errorf("ip is used for egress purposes, can not use it for a service, ip tags: %v", ip.Tags)
		}
	}

	serviceTag := tags.BuildClusterServiceFQNTag(clusterID, s.GetNamespace(), s.GetName())
	newTags := ip.Tags
	newTags = append(newTags, serviceTag)
	klog.Infof("use fixed ip in cluster, ip %s, oldTags: %v, newTags: %v", *ip.Ipaddress, ip.Tags, newTags)
	iu := &models.V1IPUpdateRequest{
		Ipaddress: ip.Ipaddress,
		Tags:      newTags,
	}
	resp, err := l.client.IP().UpdateIP(metalip.NewUpdateIPParams().WithBody(iu), nil)
	return resp.Payload, err
}

func (l *LoadBalancerController) acquireIP(service *v1.Service) (string, error) {
	annotations := service.GetAnnotations()
	addressPool, ok := annotations[constants.MetalLBSpecificAddressPool]
	if !ok {
		return l.acquireIPFromDefaultExternalNetwork(service)
	}
	return l.acquireIPFromSpecificNetwork(service, addressPool)
}

func (l *LoadBalancerController) acquireIPFromDefaultExternalNetwork(service *v1.Service) (string, error) {
	return l.acquireIPFromSpecificNetwork(service, l.defaultExternalNetworkID)
}

func (l *LoadBalancerController) acquireIPFromSpecificNetwork(service *v1.Service, addressPoolName string) (string, error) {
	nwID := strings.TrimSuffix(addressPoolName, "-"+models.V1IPBaseTypeEphemeral)
	nwID = strings.TrimSuffix(nwID, "-"+models.V1IPBaseTypeEphemeral)
	ip, err := metal.AllocateIP(l.client, *service, constants.IPPrefix, l.projectID, nwID, l.clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to acquire IPs for project %q in network %q: %w", l.projectID, nwID, err)
	}

	klog.Infof("acquired ip in network %q: %v", nwID, *ip.Ipaddress)

	return *ip.Ipaddress, nil
}

func (l *LoadBalancerController) updateLoadBalancerConfig(nodes []v1.Node) error {
	ips, err := metal.FindClusterIPs(l.client, l.projectID, l.clusterID)
	if err != nil {
		return fmt.Errorf("could not find ips of this project's cluster: %w", err)
	}

	config := newMetalLBConfig(l.defaultExternalNetworkID)
	err = config.CalculateConfig(ips, l.additionalNetworks, nodes)
	if err != nil {
		return err
	}
	err = config.Write(l.K8sClient)
	if err != nil {
		return err
	}
	return nil
}
