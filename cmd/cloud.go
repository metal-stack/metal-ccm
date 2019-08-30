package cmd

import (
	"io"
	"os"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/pkg/errors"
	"k8s.io/component-base/logs"

	"github.com/metal-pod/metal-ccm/pkg/controllers/housekeeping"
	"github.com/metal-pod/metal-ccm/pkg/controllers/instances"
	"github.com/metal-pod/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-pod/metal-ccm/pkg/controllers/zones"
	"github.com/metal-pod/metal-ccm/pkg/resources/constants"

	"k8s.io/client-go/informers"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	client *metalgo.Driver
)

type cloud struct {
	instances    *instances.InstancesController
	zones        *zones.ZonesController
	loadBalancer *loadbalancer.LoadBalancerController
}

func newCloud(_ io.Reader) (cloudprovider.Interface, error) {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm | ")
	url := os.Getenv(constants.MetalAPIUrlEnvVar)
	token := os.Getenv(constants.MetalAuthTokenEnvVar)
	hmac := os.Getenv(constants.MetalAuthHMACEnvVar)
	projectID := os.Getenv(constants.MetalProjectIDEnvVar)
	partitionID := os.Getenv(constants.MetalPartitionIDEnvVar)

	if projectID == "" {
		return nil, errors.Errorf("environment variable %q is required", constants.MetalProjectIDEnvVar)
	}

	if partitionID == "" {
		return nil, errors.Errorf("environment variable %q is required", constants.MetalPartitionIDEnvVar)
	}

	if url == "" {
		return nil, errors.Errorf("environment variable %q is required", constants.MetalAPIUrlEnvVar)
	}

	if (token == "") == (hmac == "") {
		return nil, errors.Errorf("environment variable %q or %q is required", constants.MetalAuthTokenEnvVar, constants.MetalAuthHMACEnvVar)
	}

	var err error
	client, err = metalgo.NewDriver(url, token, hmac)
	if err != nil {
		return nil, errors.Errorf("unable to initialize metal ccm:%v", err)
	}

	instancesController := instances.New(client)
	zonesController := zones.New(client)
	loadBalancerController := loadbalancer.New(client, partitionID, projectID)

	logger.Println("initialized cloud controller manager")
	return &cloud{
		instances:    instancesController,
		zones:        zonesController,
		loadBalancer: loadBalancerController,
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(constants.ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	k8sClient := clientBuilder.ClientOrDie("metal-cloud-controller-manager")
	sharedInformer := informers.NewSharedInformerFactory(k8sClient, 0)
	sharedInformer.Start(nil)
	sharedInformer.WaitForCacheSync(nil)

	housekeeper := housekeeping.New(client, stop)
	housekeeper.K8sClient = k8sClient

	c.instances.K8sClient = k8sClient
	c.loadBalancer.K8sClient = k8sClient
	c.zones.K8sClient = k8sClient

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
	return false
}
