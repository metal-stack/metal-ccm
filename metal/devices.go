package metal

import (
	"context"
	"fmt"
	"log"
	"strings"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

type instances struct {
	client  *metalgo.Driver
	project string
}

func newInstances(client *metalgo.Driver, projectID string) cloudprovider.Instances {
	return &instances{client, projectID}
}

// NodeAddresses returns the addresses of the specified instance.
func (i *instances) NodeAddresses(_ context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	log.Printf("nodeaddress:%s", name)
	device, err := deviceByName(i.client, name)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(device)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (i *instances) NodeAddressesByProviderID(_ context.Context, providerID string) ([]v1.NodeAddress, error) {
	log.Printf("nodeaddress providerID:%s", providerID)
	device, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(device)
}

func nodeAddresses(device *metalgo.MachineGetResponse) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	for _, nw := range device.Machine.Allocation.Networks {
		if *nw.Primary {
			addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: nw.Ips[0]})
			addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: nw.Ips[0]})
		} else {
			for _, ip := range nw.Ips {
				addresses = append(addresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: ip})
			}
		}
	}
	return addresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
func (i *instances) InstanceID(_ context.Context, nodeName types.NodeName) (string, error) {
	log.Printf("instanceID:%s", nodeName)
	device, err := deviceByName(i.client, nodeName)
	if err != nil {
		return "", err
	}

	return *device.Machine.ID, nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(_ context.Context, nodeName types.NodeName) (string, error) {
	log.Printf("instanceType:%s", nodeName)
	device, err := deviceByName(i.client, nodeName)
	if err != nil {
		return "", err
	}

	return *device.Machine.Size.ID, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(_ context.Context, providerID string) (string, error) {
	log.Printf("instanceType providerID:%s", providerID)
	device, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return "", err
	}

	return *device.Machine.Size.ID, nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *instances) AddSSHKeyToAllInstances(_ context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *instances) CurrentNodeName(_ context.Context, nodeName string) (types.NodeName, error) {
	log.Printf("currentNodeName:%s", nodeName)
	return types.NodeName(nodeName), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i *instances) InstanceExistsByProviderID(_ context.Context, providerID string) (bool, error) {
	log.Printf("instanceExists providerID:%s", providerID)
	machine, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return false, err
	}

	if machine.Machine.Allocation != nil {
		return true, nil
	}
	return false, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *instances) InstanceShutdownByProviderID(_ context.Context, providerID string) (bool, error) {
	log.Printf("instanceShutdown providerID:%s", providerID)
	device, err := i.deviceFromProviderID(providerID)
	if err != nil {
		return false, err
	}
	if device.Machine.Allocation == nil {
		return true, nil
	}
	if device.Machine.Events != nil && device.Machine.Events.Log != nil && len(device.Machine.Events.Log) > 0 {
		lastEvent := device.Machine.Events.Log[0].Event
		return *lastEvent != "Phoned Home", nil
	}
	return false, nil
}

func deviceByID(client *metalgo.Driver, id string) (*metalgo.MachineGetResponse, error) {
	log.Printf("deviceByID :%s", id)
	device, err := client.MachineGet(string(id))
	if err != nil {
		return nil, err
	}

	return device, nil
}

// deviceByName returns an instance where hostname matches the kubernetes node.Name
func deviceByName(client *metalgo.Driver, nodeName types.NodeName) (*metalgo.MachineGetResponse, error) {
	log.Printf("deviceByName:%s", nodeName)
	machineName := string(nodeName)
	mfr := &metalgo.MachineFindRequest{
		AllocationName: &machineName,
	}
	machines, err := client.MachineFind(mfr)
	if err != nil {
		return nil, err
	}
	if len(machines.Machines) == 0 {
		return nil, fmt.Errorf("no machine with name:%s found", nodeName)
	}
	if len(machines.Machines) > 1 {
		return nil, fmt.Errorf("more than one (%d) machine with name:%s found", len(machines.Machines), nodeName)
	}

	response := &metalgo.MachineGetResponse{
		Machine: machines.Machines[0],
	}
	return response, nil
}

// deviceIDFromProviderID returns a device's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: metal://device-id
func deviceIDFromProviderID(providerID string) (string, error) {
	log.Printf("deviceFromProvider:%s", providerID)
	if providerID == "" {
		return "", errors.New("providerID cannot be empty string")
	}

	split := strings.Split(providerID, "://")
	if len(split) != 2 {
		return "", errors.Errorf("unexpected providerID format: %s, format should be: metal://device-id", providerID)
	}

	if split[0] != providerName {
		return "", errors.Errorf("provider name from providerID should be metal: %s", providerID)
	}

	return split[1], nil
}

// deviceFromProviderID uses providerID to get the device id and return the device
func (i *instances) deviceFromProviderID(providerID string) (*metalgo.MachineGetResponse, error) {
	id, err := deviceIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return deviceByID(i.client, id)
}
