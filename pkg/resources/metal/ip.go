package metal

import (
	"fmt"

	metalgo "github.com/metal-pod/metal-go"
	"github.com/metal-pod/metal-go/api/models"

	"github.com/google/uuid"
)

// FindProjectIPs returns the IPs of the given project.
func FindProjectIPs(client *metalgo.Driver, projectID string) ([]*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &projectID,
	}

	resp, err := client.IPFind(req)
	if err != nil {
		return nil, err
	}

	return resp.IPs, nil
}

// IPAddressesOfIPs returns the IP address strings of the given ips.
func IPAddressesOfIPs(ips []*models.V1IPResponse) []string {
	var result []string
	for _, ip := range ips {
		result = append(result, *ip.Ipaddress)
	}
	return result
}

// DeleteIP deletes the given IP address.
func DeleteIP(client *metalgo.Driver, ip string) error {
	_, err := client.IPDelete(ip)
	if err != nil {
		return err
	}
	return nil
}

// AcquireIPs acquires a given count of IPs within the given network for a given project.
func AcquireIPs(client *metalgo.Driver, namePrefix, project, network string, count int) ([]*models.V1IPResponse, error) {
	var ips []*models.V1IPResponse
	for i := 0; i < count; i++ {
		name, err := uuid.NewUUID()
		if err != nil {
			return nil, err
		}

		req := &metalgo.IPAcquireRequest{
			Projectid: project,
			Networkid: network,
			Name:      fmt.Sprintf("%s%s", namePrefix, name.String()[:5]),
		}

		resp, err := client.IPAcquire(req)
		if err != nil {
			return nil, err
		}

		ips = append(ips, resp.IP)
	}

	return ips, nil
}
