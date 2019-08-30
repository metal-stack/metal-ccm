package metal

import (
	"io"
	"log"
	"os"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/pkg/errors"
	"k8s.io/client-go/informers"
	"k8s.io/component-base/logs"

	cloudprovider "k8s.io/cloud-provider"
)

const (
	metalAPIUrlEnvVar    = "METAL_API_URL"
	metalAuthTokenEnvVar = "METAL_AUTH_TOKEN"
	metalAuthHMACEnvVar  = "METAL_AUTH_HMAC"
	providerName         = "metal"
)

type cloud struct {
	client                 *metalgo.Driver
	machines               cloudprovider.Instances
	zones                  cloudprovider.Zones
	resources              *resources
	loadBalancerController *loadBalancerController
	stop                   <-chan struct{}
	logger                 *log.Logger
}

func newCloud(_ io.Reader) (cloudprovider.Interface, error) {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm cloud | ")
	url := os.Getenv(metalAPIUrlEnvVar)
	token := os.Getenv(metalAuthTokenEnvVar)
	hmac := os.Getenv(metalAuthHMACEnvVar)

	if url == "" {
		return nil, errors.Errorf("environment variable %q is required", metalAPIUrlEnvVar)
	}

	if (token == "") == (hmac == "") {
		return nil, errors.Errorf("environment variable %q or %q is required", metalAuthTokenEnvVar, metalAuthHMACEnvVar)
	}

	client, err := metalgo.NewDriver(url, token, hmac)
	if err != nil {
		return nil, errors.Errorf("unable to initialize metal ccm:%v", err)
	}

	machines := newMachines(client)
	zones := newZones(client)
	resources := newResources(client)
	loadBalancerController := newLoadBalancerController(resources)
	logger.Println("initialized")
	return &cloud{
		client:                 client,
		machines:               machines,
		zones:                  zones,
		resources:              resources,
		loadBalancerController: loadBalancerController,
		logger:                 logger,
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	clientset := clientBuilder.ClientOrDie("cloud-controller-manager-nodelister")
	sharedInformer := informers.NewSharedInformerFactory(clientset, 0)

	sharedInformer.Start(nil)
	sharedInformer.WaitForCacheSync(nil)

	c.stop = stop

	resctl := NewResourcesController(c.resources, clientset)
	err := resctl.syncMachineTagsToNodeLabels()
	if err != nil {
		c.logger.Println(err.Error())
	}
	c.loadBalancerController.resctl = resctl
	go resctl.Run(c.stop)
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancerController, true
}

// Instances returns an machines interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.machines, true
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
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set.
func (c *cloud) HasClusterID() bool {
	return false
}
