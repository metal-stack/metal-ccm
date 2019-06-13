package metal

import (
	"io"
	"os"
	"k8s.io/component-base/logs"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/pkg/errors"

	cloudprovider "k8s.io/cloud-provider"
)

const (
	metalAPIUrlEnvVar    string = "METAL_API_URL"
	metalAuthTokenEnvVar string = "METAL_AUTH_TOKEN"
	metalAuthHMACEnvVar  string = "METAL_AUTH_HMAC"
	metalProjectIDEnvVar string = "METAL_PROJECT_ID"
	providerName         string = "metal"
)

type cloud struct {
	client    *metalgo.Driver
	instances cloudprovider.Instances
	zones     cloudprovider.Zones
}

func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	logger := logs.NewLogger("metal-ccm")
	url := os.Getenv(metalAPIUrlEnvVar)
	token := os.Getenv(metalAuthTokenEnvVar)
	hmac := os.Getenv(metalAuthHMACEnvVar)
	project := os.Getenv(metalProjectIDEnvVar)

	if url == "" {
		return nil, errors.Errorf("environment variable %q is required", metalAPIUrlEnvVar)
	}

	if (token == "") == (hmac == "") {
		return nil, errors.Errorf("environment variable %q or %q is required", metalAuthTokenEnvVar, metalAuthHMACEnvVar)
	}

	if project == "" {
		return nil, errors.Errorf("environment variable %q is required", metalProjectIDEnvVar)
	}

	client, err := metalgo.NewDriver(url, token, hmac)
	if err != nil {
		return nil, errors.Errorf("unable to initialize metal ccm:%v", err)
	}

	logger.Print("metal-ccm initialized")
	return &cloud{
		client:    client,
		instances: newInstances(client, project),
		zones:     newZones(client, project),
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
} 

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
// FIXME implement
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
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
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (c *cloud) HasClusterID() bool {
	return false
}
