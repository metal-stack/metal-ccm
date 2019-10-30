package metal

import (
	"fmt"

	"github.com/metal-pod/metal-ccm/pkg/resources/constants"
	metalgo "github.com/metal-pod/metal-go"
	"github.com/metal-pod/metal-go/api/models"
	v1 "k8s.io/api/core/v1"

	"github.com/google/uuid"
)

// FindClusterIPs returns the IPs of the given cluster.
func FindClusterIPs(client *metalgo.Driver, projectID, clusterID string) ([]*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &projectID,
		ClusterID: &clusterID,
	}

	resp, err := client.IPFind(req)
	if err != nil {
		return nil, err
	}

	return resp.IPs, nil
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

// FindClusterIPsWithTag returns the IPs of the given cluster that also have the given tag.
func FindClusterIPsWithTag(client *metalgo.Driver, projectID, clusterID string, tag string) ([]*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &projectID,
		ClusterID: &clusterID,
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
func AllocateIP(client *metalgo.Driver, svc v1.Service, namePrefix, project, network, clusterID, clusterName string) (*models.V1IPResponse, error) {
	name, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	req := &metalgo.IPAllocateRequest{
		Name:      fmt.Sprintf("%s%s", namePrefix, name.String()[:5]),
		Projectid: project,
		Networkid: network,
		Clusterid: &clusterID,
		Type:      "ephemeral",
		Tags:      []string{GenerateServiceTag(clusterID, svc)},
	}

	resp, err := client.IPAllocate(req)
	if err != nil {
		return nil, err
	}

	return resp.IP, nil
}

// TagIP associates an IP with an cluster
func TagIP(client *metalgo.Driver, address, cluster, project string, tags []string) (*metalgo.IPDetailResponse, error) {
	it := &metalgo.IPTagRequest{
		IPAddress: address,
		ClusterID: &cluster,
		Tags:      tags,
	}
	return client.IPTag(it)
}

func GenerateServiceTag(clusterID string, s v1.Service) string {
	return fmt.Sprintf("%s=%s", constants.TagServicePrefix, fmt.Sprintf("%s/%s/%s", clusterID, s.GetNamespace(), s.GetName()))
}

func GenerateClusterTag(clusterID string) string {
	return fmt.Sprintf("%s/clusterid=%s", constants.TagClusterPrefix, clusterID)
}
