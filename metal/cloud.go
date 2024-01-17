package metal

import (
	"fmt"
	"io"
	"os"
	"strings"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/rest"

	"github.com/metal-stack/metal-ccm/pkg/controllers/housekeeping"
	"github.com/metal-stack/metal-ccm/pkg/controllers/instances"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-ccm/pkg/controllers/zones"
	"github.com/metal-stack/metal-ccm/pkg/resources/constants"
	"github.com/metal-stack/metal-ccm/pkg/resources/metal"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

var (
	client metalgo.Client
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
	client, err = metalgo.NewDriver(url, token, hmac)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize metal ccm:%w", err)
	}

	resp, err := client.Health().Health(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("metal-api health endpoint not reachable:%w", err)
	}
	if resp.Payload != nil && resp.Payload.Status != nil && *resp.Payload.Status != string(rest.HealthStatusHealthy) {
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
	projectID := os.Getenv(constants.MetalProjectIDEnvVar)
	sshPublicKey := os.Getenv(constants.MetalSSHPublicKey)
	clusterID := os.Getenv(constants.MetalClusterIDEnvVar)

	k8sClient := clientBuilder.ClientOrDie("cloud-controller-manager")

	housekeeper := housekeeping.New(client, stop, c.loadBalancer, k8sClient, projectID, sshPublicKey, clusterID)
	ms := metal.New(client, k8sClient, projectID)

	c.instances.MetalService = ms
	c.loadBalancer.K8sClient = k8sClient
	c.loadBalancer.MetalService = ms
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
