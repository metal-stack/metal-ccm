package metal

import (
	"fmt"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
)

// FindExternalNetworksInPartition returns the external networks of a partition.
func FindExternalNetworksInPartition(client *metalgo.Driver, partitionID string) ([]*models.V1NetworkResponse, error) {
	falseFlag := false
	nfr := &metalgo.NetworkFindRequest{
		PartitionID:  &partitionID,
		PrivateSuper: &falseFlag,
		Underlay:     &falseFlag,
	}

	resp, err := client.NetworkFind(nfr)
	if err != nil {
		return nil, fmt.Errorf("unable to find network(s): %v", err)
	}
	return resp.Networks, nil
}

// ListNetworks returns all networks.
func ListNetworks(client *metalgo.Driver) ([]*models.V1NetworkResponse, error) {
	resp, err := client.NetworkList()
	if err != nil {
		return nil, err
	}
	return resp.Networks, nil
}

// NetworksByID returns networks as map with their ID as the key.
func NetworksByID(nws []*models.V1NetworkResponse) map[string]*models.V1NetworkResponse {
	result := make(map[string]*models.V1NetworkResponse, len(nws))
	for i := range nws {
		result[*nws[i].ID] = nws[i]
	}
	return result
}
