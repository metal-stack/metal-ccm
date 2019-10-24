package metal

import (
	"fmt"
	"strings"

	"github.com/metal-pod/metal-ccm/pkg/resources/constants"
	metalgo "github.com/metal-pod/metal-go"
	"github.com/metal-pod/metal-go/api/models"

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

// FindAvailableProjectIP returns a free IP of the given project.
func FindAvailableProjectIP(client *metalgo.Driver, projectID string) (*models.V1IPResponse, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &projectID,
	}
	resp, err := client.IPFind(req)
	if err != nil {
		return nil, err
	}

	for _, i := range resp.IPs {
		occupied := false
		for _, t := range i.Tags {
			if strings.HasPrefix(t, constants.TagClusterPrefix) || strings.HasPrefix(t, constants.TagMachinePrefix) {
				occupied = true
				break
			}
		}
		if !occupied {
			return i, nil
		}
	}

	return nil, fmt.Errorf("no ip available for project")
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

// AcquireIP acquires an IP within the given network for a given project.
func AcquireIP(client *metalgo.Driver, namePrefix, project, network, cluster string) (*models.V1IPResponse, error) {
	name, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	req := &metalgo.IPAcquireRequest{
		Name:      fmt.Sprintf("%s%s", namePrefix, name.String()[:5]),
		Projectid: project,
		Networkid: network,
		Clusterid: &cluster,
		Type:      "ephemeral",
	}

	resp, err := client.IPAcquire(req)
	if err != nil {
		return nil, err
	}

	return resp.IP, nil
}

// AssociateIP associates an IP with an cluster
func AssociateIP(client *metalgo.Driver, address, cluster, project string, tags []string) (*metalgo.IPDetailResponse, error) {
	iuc := &metalgo.IPUseInClusterRequest{
		IPAddress: address,
		ClusterID: cluster,
		ProjectID: project,
		Tags:      tags,
	}
	return client.IPUseInCluster(iuc)
}

// DeassociateIP associates an IP with an cluster
func DeassociateIP(client *metalgo.Driver, address, cluster, project string, tags []string) (*metalgo.IPDetailResponse, error) {
	irc := &metalgo.IPReleaseFromClusterRequest{
		IPAddress: address,
		ClusterID: cluster,
		ProjectID: project,
		Tags:      tags,
	}
	return client.IPReleaseFromCluster(irc)
}
