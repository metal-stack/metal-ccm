package zones

import (
	"context"
	"strings"

	"github.com/metal-stack/metal-ccm/pkg/resources/metal"

	metalgo "github.com/metal-stack/metal-go"

	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
)

type ZonesController struct {
	client    metalgo.Client
	K8sClient clientset.Interface
	ms        *metal.MetalService
}

var (
	noZone = cloudprovider.Zone{}
)

// New returns a new zones controller that satisfies the kubernetes cloud provider zones interface
func New(client metalgo.Client) *ZonesController {
	return &ZonesController{
		client: client,
		ms:     metal.New(client),
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
func (z ZonesController) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	machine, err := z.ms.GetMachineFromProviderID(ctx, providerID)
	if err != nil {
		return noZone, err
	}

	// TODO: check if failureDomain == Partition
	return cloudprovider.Zone{
		FailureDomain: *machine.Partition.ID,
		Region:        getRegionFromPartitionID(machine.Partition.ID),
	}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name.
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z ZonesController) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	machine, err := z.ms.GetMachineFromNode(ctx, nodeName)
	if err != nil {
		return noZone, err
	}

	// TODO: check if failureDomain == Partition
	return cloudprovider.Zone{
		FailureDomain: *machine.Partition.ID,
		Region:        getRegionFromPartitionID(machine.Partition.ID),
	}, nil
}

// getRegionFromPartitionID extracts the region from a given partitionID
func getRegionFromPartitionID(partitionID *string) string {
	// if partitionID contains a hyphen, return part before first hyphen as region, otherwise return partitionID
	split := strings.Split(*partitionID, "-")
	return split[0]
}
