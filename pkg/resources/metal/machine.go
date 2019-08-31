package metal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/metal-pod/metal-ccm/pkg/resources/constants"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/metal-pod/metal-go/api/models"

	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"
)

// GetMachinesFromNodes gets metal machines from K8s nodes.
func GetMachinesFromNodes(client *metalgo.Driver, nodes []*v1.Node) ([]*models.V1MachineResponse, error) {
	var mm []*models.V1MachineResponse
	for _, n := range nodes {
		m, err := GetMachineFromNode(client, types.NodeName(n.Name))
		if err != nil {
			return nil, err
		}
		mm = append(mm, m)
	}

	return mm, nil
}

// GetMachineFromNode returns a machine where hostname matches the kubernetes node.Name.
func GetMachineFromNode(client *metalgo.Driver, nodeName types.NodeName) (*models.V1MachineResponse, error) {
	machineHostname := string(nodeName)
	// if strings.HasPrefix(machineHostname, "kind-worker") {
	// 	return getTestMachine(client)
	// }

	mfr := &metalgo.MachineFindRequest{
		AllocationHostname: &machineHostname,
	}
	machines, err := client.MachineFind(mfr)
	if err != nil {
		return nil, err
	}
	if len(machines.Machines) == 0 {
		return nil, fmt.Errorf("no machine with name %q found", nodeName)
	}
	if len(machines.Machines) > 1 {
		return nil, fmt.Errorf("more than one (%d) machine with name %q found", len(machines.Machines), nodeName)
	}

	return machines.Machines[0], nil
}

// GetMachineFromProviderID uses providerID to get the machine id and returns the machine.
func GetMachineFromProviderID(client *metalgo.Driver, providerID string) (*models.V1MachineResponse, error) {
	id, err := decodeMachineIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return GetMachine(client, id)
}

// machineIDFromProviderID returns a machine's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: metal://machine-id.
func decodeMachineIDFromProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", errors.New("providerID cannot be empty")
	}

	split := strings.Split(providerID, "://")
	if len(split) != 2 {
		return "", fmt.Errorf("unexpected providerID format %q, format should be %q", providerID, "metal://<machine-id>")
	}

	if split[0] != constants.ProviderName {
		return "", fmt.Errorf("provider name from providerID %q should be metal", providerID)
	}

	return split[1], nil
}

// GetMachine returns a metal machine by its ID.
func GetMachine(client *metalgo.Driver, id string) (*models.V1MachineResponse, error) {
	// if strings.HasPrefix(id, "kind-worker") {
	// 	return getTestMachine(client)
	// }

	machine, err := client.MachineGet(id)
	if err != nil {
		return nil, err
	}

	return machine.Machine, nil
}

// func getTestMachine(client *metalgo.Driver) (*metalgo.MachineGetResponse, error) {
// 	m, err := client.MachineGet("4fde6800-710d-11e9-8000-efbeaddeefbe")
// 	if err != nil {
// 		return nil, err
// 	}
// 	m.Machine.Tags = []string{
// 		fmt.Sprintf("%s=test", projectIDTag),
// 	}
// 	return m, nil
// }
