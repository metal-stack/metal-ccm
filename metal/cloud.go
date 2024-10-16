package metal

import (
	"fmt"
	"io"
	"os"
	"strings"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/healthstatus"

	"github.com/metal-stack/metal-ccm/pkg/controllers/housekeeping"
	"github.com/metal-stack/metal-ccm/pkg/controllers/instances"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer/cilium"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer/metallb"
	"github.com/metal-stack/metal-ccm/pkg/controllers/zones"
	"github.com/metal-stack/metal-ccm/pkg/resources/constants"
	"github.com/metal-stack/metal-ccm/pkg/resources/metal"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
)

var (
	metalclient metalgo.Client
)

type cloud struct {
	instances    *instances.InstancesController
	zones        *zones.ZonesController
	loadBalancer *loadbalancer.LoadBalancerController
}

func NewCloud(_ io.Reader) (cloudprovider.Interface, error) {
	url := os.Getenv(constants.MetalAPIUrlEnvVar)
	token := os.Getenv(constants.MetalAuthTokenEnvVar)
	hmac := os.Getenv(constants.MetalAuthHMACEnvVar)
	projectID := os.Getenv(constants.MetalProjectIDEnvVar)
	partitionID := os.Getenv(constants.MetalPartitionIDEnvVar)
	clusterID := os.Getenv(constants.MetalClusterIDEnvVar)
	defaultExternalNetworkID := os.Getenv(constants.MetalDefaultExternalNetworkEnvVar)

	var (
		additionalNetworksString = os.Getenv(constants.MetalAdditionalNetworks)
		additionalNetworks       []string
	)
	for _, n := range strings.Split(additionalNetworksString, ",") {
		n := strings.TrimSpace(n)
		if n != "" {
			additionalNetworks = append(additionalNetworks, n)
		}
	}

	if projectID == "" {
		return nil, fmt.Errorf("environment variable %q is required", constants.MetalProjectIDEnvVar)
	}

	if partitionID == "" {
		return nil, fmt.Errorf("environment variable %q is required", constants.MetalPartitionIDEnvVar)
	}

	if clusterID == "" {
		return nil, fmt.Errorf("environment variable %q is required", constants.MetalClusterIDEnvVar)
	}

	if url == "" {
		return nil, fmt.Errorf("environment variable %q is required", constants.MetalAPIUrlEnvVar)
	}

	if (token == "") == (hmac == "") {
		return nil, fmt.Errorf("environment variable %q or %q is required", constants.MetalAuthTokenEnvVar, constants.MetalAuthHMACEnvVar)
	}

	var err error
	metalclient, err = metalgo.NewDriver(url, token, hmac)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize metal ccm:%w", err)
	}

	resp, err := metalclient.Health().Health(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("metal-api health endpoint not reachable:%w", err)
	}
	if resp.Payload != nil && resp.Payload.Status != nil && *resp.Payload.Status != string(healthstatus.HealthStatusHealthy) {
		return nil, fmt.Errorf("metal-api not healthy, restarting")
	}

	instancesController := instances.New(defaultExternalNetworkID)
	zonesController := zones.New()
	loadBalancerController := loadbalancer.New(partitionID, projectID, clusterID, defaultExternalNetworkID, additionalNetworks)

	klog.Info("initialized cloud controller manager")
	return &cloud{
		instances:    instancesController,
		zones:        zonesController,
		loadBalancer: loadBalancerController,
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	projectID := os.Getenv(constants.MetalProjectIDEnvVar)
	sshPublicKey := os.Getenv(constants.MetalSSHPublicKey)
	clusterID := os.Getenv(constants.MetalClusterIDEnvVar)
	loadbalancerType := os.Getenv(constants.Loadbalancer)

	k8sClientSet := clientBuilder.ClientOrDie("cloud-controller-manager")
	k8sRestConfig, err := clientBuilder.Config("cloud-controller-manager")
	if err != nil {
		klog.Fatalf("unable to get k8s rest config: %v", err)
	}
	k8sRestConfig.ContentType = "application/json"
	err = metallbv1beta1.AddToScheme(scheme)
	if err != nil {
		klog.Fatalf("unable to add metallb v1beta1 to scheme: %v", err)
	}
	err = metallbv1beta2.AddToScheme(scheme)
	if err != nil {
		klog.Fatalf("unable to add metallb v1beta2 to scheme: %v", err)
	}
	k8sClient, err := client.New(k8sRestConfig, client.Options{Scheme: scheme})
	if err != nil {
		klog.Fatalf("unable to create k8s client: %v", err)
	}

	var config loadbalancer.LoadBalancerConfig
	switch loadbalancerType {
	case "metallb":
		config = metallb.NewMetalLBConfig()
	case "cilium":
		config = cilium.NewCiliumConfig(k8sClientSet)
	default:
		config = metallb.NewMetalLBConfig()
	}

	housekeeper := housekeeping.New(metalclient, stop, c.loadBalancer, k8sClientSet, projectID, sshPublicKey, clusterID)
	ms := metal.New(metalclient, k8sClientSet, projectID)

	c.instances.MetalService = ms
	c.loadBalancer.K8sClientSet = k8sClientSet
	c.loadBalancer.K8sClient = k8sClient
	c.loadBalancer.MetalService = ms
	c.loadBalancer.LoadBalancerConfig = config
	c.zones.MetalService = ms

	go housekeeper.Run()
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancer, true
}

// Instances returns an machines interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// Instances returns an machines interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return c.instances, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return c.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	return constants.ProviderName
}

// HasClusterID returns true if a ClusterID is required and set.
func (c *cloud) HasClusterID() bool {
	return true
}
