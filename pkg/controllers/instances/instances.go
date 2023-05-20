package instances

import (
	"context"
	"fmt"

	"github.com/metal-stack/metal-ccm/pkg/resources/metal"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	mn "github.com/metal-stack/metal-lib/pkg/net"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type InstancesController struct {
	client                 metalgo.Client
	K8sClient              clientset.Interface
	defaultExternalNetwork string
	ms                     *metal.MetalService
}

// New returns a new instance controller that satisfies the kubernetes cloud provider instances interface
func New(client metalgo.Client, defaultExternalNetwork string) *InstancesController {
	return &InstancesController{
		client:                 client,
		defaultExternalNetwork: defaultExternalNetwork,
		ms:                     metal.New(client),
	}
}

// NodeAddresses returns the addresses of the specified instance.
func (i *InstancesController) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.Infof("NodeAddresses: nodeName %q", name)
	machine, err := i.ms.GetMachineFromNode(ctx, name)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(machine, i.defaultExternalNetwork)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose node addresses are being queried. m.e. local metadata
// services cannot be used in this method to obtain node addresses.
func (i *InstancesController) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.Infof("NodeAddressesByProviderID: providerID %q", providerID)
	machine, err := i.ms.GetMachineFromProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(machine, i.defaultExternalNetwork)
}

func nodeAddresses(machine *models.V1MachineResponse, defaultExternalNetwork string) ([]v1.NodeAddress, error) {
	if machine == nil || machine.Allocation == nil {
		return nil, nil
	}

	var addresses []v1.NodeAddress
	for _, nw := range machine.Allocation.Networks {
		if nw == nil || nw.Networktype == nil {
			continue
		}
		// The primary private network either shared or unshared
		if *nw.Networktype == mn.PrivatePrimaryUnshared || *nw.Networktype == mn.PrivatePrimaryShared {
			if len(nw.Ips) == 0 {
				continue
			}
			addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: *machine.Allocation.Hostname})
			addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: nw.Ips[0]})
			continue
		}

		if *nw.Networkid == defaultExternalNetwork {
			for _, ip := range nw.Ips {
				addresses = append(addresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: ip})
			}
		}
	}
	return addresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound).
func (i *InstancesController) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.Infof("InstanceID: nodeName %q", nodeName)
	machine, err := i.ms.GetMachineFromNode(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return *machine.ID, nil
}

// InstanceType returns the type of the specified instance.
func (i *InstancesController) InstanceType(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.Infof("InstanceType: nodeName %q", nodeName)
	machine, err := i.ms.GetMachineFromNode(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return *machine.Size.ID, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *InstancesController) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.Infof("InstanceTypeByProviderID: providerID %q", providerID)
	machine, err := i.ms.GetMachineFromProviderID(ctx, providerID)
	if err != nil {
		return "", err
	}

	return *machine.Size.ID, nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all machines.
// Expected format for the key is standard ssh-keygen format: <protocol> <blob>.
func (i *InstancesController) AddSSHKeyToAllInstances(_ context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on.
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname.
func (i *InstancesController) CurrentNodeName(_ context.Context, nodeName string) (types.NodeName, error) {
	klog.Infof("CurrentNodeName: nodeName %q", nodeName)
	return types.NodeName(nodeName), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for machines that exist but are stopped/sleeping.
func (i *InstancesController) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.Infof("InstanceExistsByProviderID: providerID %q", providerID)
	machine, err := i.ms.GetMachineFromProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return machine.Allocation != nil, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider.
func (i *InstancesController) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.Infof("InstanceShutdownByProviderID: providerID %q", providerID)
	machine, err := i.ms.GetMachineFromProviderID(ctx, providerID)
	if err != nil || machine.Allocation == nil {
		return true, err
	}
	return false, nil
}

// ------------- InstanceV2 interface functions ---------------------------

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *InstancesController) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	klog.Infof("InstanceExists: node %q", node.GetName())
	machine, err := i.ms.GetMachineFromProviderID(ctx, node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	return machine.Allocation != nil, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *InstancesController) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	klog.Infof("InstanceShutdown: node %q", node.GetName())
	machine, err := i.ms.GetMachineFromProviderID(ctx, node.Spec.ProviderID)
	if err != nil || machine.Allocation == nil {
		return true, err
	}
	return false, nil
}

// InstanceMetadata contains metadata about a specific instance.
// Values returned in InstanceMetadata are translated into specific fields and labels for Node.
//
// ProviderID is a unique ID used to identify an instance on the cloud provider.
// The ProviderID set here will be set on the node's spec.providerID field.
// The provider ID format can be set by the cloud provider but providers should
// ensure the format does not change in any incompatible way.
//
// The provider ID format used by existing cloud provider has been:
//
//	<provider-name>://<instance-id>
//
// Existing providers setting this field should preserve the existing format
// currently being set in node.spec.providerID.
func (i *InstancesController) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.Infof("InstanceMetadata: node %q", node.GetName())
	machine, err := i.ms.GetMachineFromProviderID(ctx, node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}

	if machine == nil {
		return nil, fmt.Errorf("machine is nil for node:%s", node.Name)
	}
	nas, err := nodeAddresses(machine, i.defaultExternalNetwork)
	if err != nil {
		return nil, err
	}
	md := &cloudprovider.InstanceMetadata{
		InstanceType:  *machine.Size.ID,
		ProviderID:    fmt.Sprintf("metal://%s/%s", *machine.Partition.ID, *machine.ID),
		NodeAddresses: nas,
	}
	return md, nil
}
