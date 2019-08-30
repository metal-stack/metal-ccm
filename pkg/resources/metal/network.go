package metal

import (
	"fmt"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/metal-pod/metal-go/api/models"
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
		return nil, fmt.Errorf("unable to find external network(s): %v", err)
	}

	return resp.Networks, nil
}
