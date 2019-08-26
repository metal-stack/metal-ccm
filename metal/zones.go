package metal

import (
	"context"

	"github.com/metal-pod/metal-go"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider"
)

type zones struct {
	client *metalgo.Driver
}

var (
	noZone = cloudprovider.Zone{}
)

func newZones(client *metalgo.Driver) cloudprovider.Zones {
	return zones{client}
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in.
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z zones) GetZone(_ context.Context) (cloudprovider.Zone, error) {
	return noZone, cloudprovider.NotImplemented
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerID.
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z zones) GetZoneByProviderID(_ context.Context, providerID string) (cloudprovider.Zone, error) {
	id, err := machineIDFromProviderID(providerID)
	if err != nil {
		return noZone, err
	}

	machine, err := machineByID(z.client, id)
	if err != nil {
		return noZone, err
	}

	// TODO: check if failureDomain == Partition
	return cloudprovider.Zone{
		FailureDomain: *machine.Machine.Partition.ID,
		Region:        *machine.Machine.Partition.ID,
	}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name.
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z zones) GetZoneByNodeName(_ context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	machine, err := machineByHostname(z.client, nodeName)
	if err != nil {
		return noZone, err
	}

	// TODO: check if failureDomain == Partition
	return cloudprovider.Zone{
		FailureDomain: *machine.Machine.Partition.ID,
		Region:        *machine.Machine.Partition.ID,
	}, nil
}
