package loadbalancer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/metal-stack/metal-ccm/pkg/tags"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"

	"github.com/metal-stack/metal-ccm/pkg/resources/constants"
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-stack/metal-ccm/pkg/resources/metal"
	"github.com/metal-stack/metal-go/api/models"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	retrygo "github.com/avast/retry-go/v4"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/util/retry"
	cloudprovider "k8s.io/cloud-provider"
)

type LoadBalancerController struct {
	MetalService             *metal.MetalService
	partitionID              string
	projectID                string
	clusterID                string
	defaultExternalNetworkID string
	additionalNetworks       sets.Set[string]
	K8sClientSet             clientset.Interface
	K8sClient                client.Client
	configWriteMutex         *sync.Mutex
	ipAllocateMutex          *sync.Mutex
	ipUpdateMutex            *sync.Mutex
}

// New returns a new load balancer controller that satisfies the kubernetes cloud provider load balancer interface
func New(partitionID, projectID, clusterID, defaultExternalNetworkID string, additionalNetworks []string) *LoadBalancerController {
	return &LoadBalancerController{
		partitionID:              partitionID,
		projectID:                projectID,
		clusterID:                clusterID,
		defaultExternalNetworkID: defaultExternalNetworkID,
		additionalNetworks:       sets.New(additionalNetworks...),
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
	klog.Infof("GetLoadBalancerName: clusterName %q, namespace %q, serviceName %q", clusterName, service.Namespace, service.Name)

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

		ip, err := l.MetalService.FindProjectIP(ctx, l.projectID, fixedIP)
		if err != nil {
			return nil, err
		}
		newIP, err := l.useIPInCluster(ctx, *ip, l.clusterID, *service)
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

	ip, err = l.acquireIP(ctx, service)
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
		_, err2 := l.MetalService.UpdateIP(ctx, &models.V1IPUpdateRequest{
			Ipaddress: &ip,
			Tags:      []string{},
		})
		if err2 != nil {
			klog.Errorf("error during ip rollback occurred: %v", err2)
			return err
		}

		err2 = l.MetalService.FreeIP(ctx, ip)
		if err2 != nil {
			klog.Errorf("error during ip rollback occurred: %v", err2)
			return err
		}

		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		s, err := l.K8sClientSet.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		s.Spec.LoadBalancerIP = ip
		_, err = l.K8sClientSet.CoreV1().Services(s.Namespace).Update(ctx, s, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return nil, rollback(err)
	}

	ingressStatus = append(ingressStatus, v1.LoadBalancerIngress{IP: ip})

	err = l.UpdateMetalLBConfig(ctx, ns)
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
	return l.UpdateMetalLBConfig(ctx, ns)
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
	klog.Infof("EnsureLoadBalancerDeleted: clusterName %q, namespace %q, serviceName %q, serviceStatus: %v", clusterName, service.Namespace, service.Name, service.Status)

	s := *service
	serviceTag := tags.BuildClusterServiceFQNTag(l.clusterID, s.GetNamespace(), s.GetName())
	ips, err := l.MetalService.FindProjectIPsWithTag(ctx, l.projectID, serviceTag)
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
				newIP, err := l.MetalService.UpdateIP(ctx, iu)
				if err != nil {
					return fmt.Errorf("could not update ip with new tags: %w", err)
				}
				klog.Infof("updated ip: %q", pointer.SafeDeref(newIP.Ipaddress))
				if *ip.Type == models.V1IPBaseTypeEphemeral && last {
					klog.Infof("freeing unused ephemeral ip: %s, tags: %s", *ip.Ipaddress, newTags)
					err := l.MetalService.FreeIP(ctx, *ip.Ipaddress)
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
func (l *LoadBalancerController) UpdateMetalLBConfig(ctx context.Context, nodes []v1.Node) error {
	l.configWriteMutex.Lock()
	defer l.configWriteMutex.Unlock()

	err := l.updateLoadBalancerConfig(ctx, nodes)
	if err != nil {
		return err
	}

	klog.Info("metallb config updated successfully")

	return nil
}

func (l *LoadBalancerController) useIPInCluster(ctx context.Context, ip models.V1IPResponse, clusterID string, s v1.Service) (*models.V1IPResponse, error) {
	tm := tag.NewTagMap(ip.Tags)

	if _, ok := tm.Value(tag.MachineID); ok {
		return nil, fmt.Errorf("ip is used for a machine, can not use it for a service, ip tags: %v", ip.Tags)
	}
	if _, ok := tm.Value(tag.ClusterEgress); ok {
		return nil, fmt.Errorf("ip is used for egress purposes, can not use it for a service, ip tags: %v", ip.Tags)
	}

	serviceTag := tags.BuildClusterServiceFQNTag(clusterID, s.GetNamespace(), s.GetName())
	newTags := ip.Tags
	newTags = append(newTags, serviceTag)
	klog.Infof("use fixed ip in cluster, ip %s, oldTags: %v, newTags: %v", *ip.Ipaddress, ip.Tags, newTags)
	iu := &models.V1IPUpdateRequest{
		Ipaddress: ip.Ipaddress,
		Tags:      newTags,
	}
	resp, err := l.MetalService.UpdateIP(ctx, iu)
	return resp, err
}

func (l *LoadBalancerController) acquireIP(ctx context.Context, service *v1.Service) (string, error) {
	annotations := service.GetAnnotations()
	addressPool, ok := annotations[constants.MetalLBSpecificAddressPool]
	if !ok {
		if l.defaultExternalNetworkID == "" {
			return "", fmt.Errorf(`no default network for ip acquisition specified, acquire an ip for your cluster's project and specify it directly in "spec.loadBalancerIP"`)
		}

		return l.acquireIPFromSpecificNetwork(ctx, service, l.defaultExternalNetworkID)
	}
	return l.acquireIPFromSpecificNetwork(ctx, service, addressPool)
}

func (l *LoadBalancerController) acquireIPFromSpecificNetwork(ctx context.Context, service *v1.Service, addressPoolName string) (string, error) {
	nwID := strings.TrimSuffix(addressPoolName, "-"+models.V1IPBaseTypeEphemeral)
	nwID = strings.TrimSuffix(nwID, "-"+models.V1IPBaseTypeEphemeral)
	ip, err := l.MetalService.AllocateIP(ctx, *service, constants.IPPrefix, l.projectID, nwID, l.clusterID)
	if err != nil {
		return "", fmt.Errorf("failed to acquire IPs for project %q in network %q: %w", l.projectID, nwID, err)
	}

	klog.Infof("acquired ip in network %q: %v", nwID, *ip.Ipaddress)

	return *ip.Ipaddress, nil
}

func (l *LoadBalancerController) updateLoadBalancerConfig(ctx context.Context, nodes []v1.Node) error {
	ips, err := l.MetalService.FindClusterIPs(ctx, l.projectID, l.clusterID)
	if err != nil {
		return fmt.Errorf("could not find ips of this project's cluster: %w", err)
	}

	config := newMetalLBConfig()
	err = config.CalculateConfig(ips, l.additionalNetworks, nodes)
	if err != nil {
		return err
	}

	// TODO: in a future release this can be removed
	err = l.K8sClient.Delete(ctx, &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      "config",
		Namespace: metallbNamespace,
	}})
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("unable to cleanup deprecated metallb configmap: %w", err)
	}

	err = config.WriteCRs(ctx, l.K8sClient)
	if err != nil {
		return err
	}
	return nil
}
