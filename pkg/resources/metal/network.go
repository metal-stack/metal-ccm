package metal

import (
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
)

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
