package zones

import (
	"context"
	"log"

	"github.com/metal-stack/metal-ccm/pkg/resources/metal"

	metalgo "github.com/metal-pod/metal-go"

	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/component-base/logs"
)

type ZonesController struct {
	client    *metalgo.Driver
	logger    *log.Logger
	K8sClient clientset.Interface
}

var (
	noZone = cloudprovider.Zone{}
)

// New returns a new zones controller that satisfies the kubernetes cloud provider zones interface
func New(client *metalgo.Driver) *ZonesController {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm zones | ")

	return &ZonesController{
		client: client,
		logger: logger,
	}
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in.
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z ZonesController) GetZone(_ context.Context) (cloudprovider.Zone, error) {
	return noZone, cloudprovider.NotImplemented
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerID.
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z ZonesController) GetZoneByProviderID(_ context.Context, providerID string) (cloudprovider.Zone, error) {
	machine, err := metal.GetMachineFromProviderID(z.client, providerID)
	if err != nil {
		return noZone, err
	}

	// TODO: check if failureDomain == Partition
	return cloudprovider.Zone{
		FailureDomain: *machine.Partition.ID,
		Region:        *machine.Partition.ID,
	}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name.
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z ZonesController) GetZoneByNodeName(_ context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	machine, err := metal.GetMachineFromNode(z.client, nodeName)
	if err != nil {
		return noZone, err
	}

	// TODO: check if failureDomain == Partition
	return cloudprovider.Zone{
		FailureDomain: *machine.Partition.ID,
		Region:        *machine.Partition.ID,
	}, nil
}
