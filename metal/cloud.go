package metal

import (
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/util/runtime"
	"os"

	"github.com/metal-pod/metal-go"
	"github.com/pkg/errors"
	"k8s.io/client-go/informers"
	"k8s.io/component-base/logs"

	"k8s.io/cloud-provider"
)

const (
	metalAPIUrlEnvVar    = "METAL_API_URL"
	metalAuthTokenEnvVar = "METAL_AUTH_TOKEN"
	metalAuthHMACEnvVar  = "METAL_AUTH_HMAC"
	providerName         = "metal"
)

type cloud struct {
	client       *metalgo.Driver
	machines     cloudprovider.Instances
	zones        cloudprovider.Zones
	resources    *resources
	stop         <-chan struct{}
	loadBalancer *loadBalancer
}

func newCloud(_ io.Reader) (cloudprovider.Interface, error) {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm cloud ")
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
	loadBalancer := newLoadBalancer(resources)
	logger.Println("initialized")
	return &cloud{
		client:       client,
		machines:     machines,
		zones:        zones,
		resources:    resources,
		loadBalancer: loadBalancer,
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
	nodeInformer := sharedInformer.Core().V1().Nodes()

	resctl := NewResourcesController(c.resources, nodeInformer, clientset)
	c.loadBalancer.resctl = resctl

	err := resctl.AddFirewallNetworkAddressPools()
	if err != nil {
		fmt.Println(err.Error())
		runtime.HandleError(err)
	}

	sharedInformer.Start(nil)
	sharedInformer.WaitForCacheSync(nil)

	c.stop = stop
	go resctl.Run(c.stop)
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancer, true
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
