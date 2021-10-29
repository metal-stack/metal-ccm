package metal

import (
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	v1 "k8s.io/api/core/v1"
)

// ListNetworksOfNodes returns the machine Networks, takes only the first node into account.
// This assumes that all worker nodes have the same machine.Allocation.Networks
func ListNetworksOfNodes(client *metalgo.Driver, nodes []v1.Node) (map[string]*models.V1MachineNetwork, error) {
	if len(nodes) < 1 {
		return nil, nil
	}
	m, err := GetMachineFromProviderID(client, nodes[0].Spec.ProviderID)
	if err != nil {
		return nil, err
	}

	if m.Allocation != nil {
		nws := m.Allocation.Networks
		result := make(map[string]*models.V1MachineNetwork, len(nws))
		for i := range nws {
			result[*nws[i].Networkid] = nws[i]
		}
		return result, nil
	}

	return nil, nil
}
