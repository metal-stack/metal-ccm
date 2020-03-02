package metal

import (
	"fmt"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	v1 "k8s.io/api/core/v1"

	"github.com/google/uuid"
)

// FindClusterIPs returns the IPs of the given cluster.
func FindClusterIPs(client *metalgo.Driver, projectID, clusterID string) ([]*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &projectID,
	}

	resp, err := client.IPFind(req)
	if err != nil {
		return nil, err
	}

	result := []*models.V1IPResponse{}
	for _, i := range resp.IPs {
		for _, t := range i.Tags {
			if metalgo.TagIsMemberOfCluster(t, clusterID) {
				result = append(result, i)
				break
			}
		}
	}

	return result, nil
}

// FindProjectIP returns the IP
func FindProjectIP(client *metalgo.Driver, projectID, ip string) (*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		IPAddress: &ip,
		ProjectID: &projectID,
	}

	resp, err := client.IPFind(req)
	if err != nil {
		return nil, err
	}

	if len(resp.IPs) != 1 {
		return nil, fmt.Errorf("ip %s is ambiguous for projectID: %s", ip, projectID)
	}

	return resp.IPs[0], nil
}

// FindProjectIPsWithTag returns the IPs of the given project that also have the given tag.
func FindProjectIPsWithTag(client *metalgo.Driver, projectID, tag string) ([]*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &projectID,
		Tags:      []string{tag},
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

// FreeIP frees the given IP address.
func FreeIP(client *metalgo.Driver, ip string) error {
	_, err := client.IPFree(ip)
	if err != nil {
		return err
	}
	return nil
}

// AllocateIP acquires an IP within the given network for a given project.
func AllocateIP(client *metalgo.Driver, svc v1.Service, namePrefix, project, network, clusterID string) (*models.V1IPResponse, error) {
	name, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	req := &metalgo.IPAllocateRequest{
		Name:      fmt.Sprintf("%s%s", namePrefix, name.String()[:5]),
		Projectid: project,
		Networkid: network,
		Type:      metalgo.IPTypeEphemeral,
		Tags:      []string{metalgo.BuildServiceTag(clusterID, svc.GetNamespace(), svc.GetName())},
	}

	resp, err := client.IPAllocate(req)
	if err != nil {
		return nil, err
	}

	return resp.IP, nil
}
