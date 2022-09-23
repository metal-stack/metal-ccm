package metal

import (
	"fmt"

	"github.com/metal-stack/metal-ccm/pkg/tags"

	metalgo "github.com/metal-stack/metal-go"
	metalip "github.com/metal-stack/metal-go/api/client/ip"

	"github.com/metal-stack/metal-go/api/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/google/uuid"
)

// FindClusterIPs returns the allowed IPs of the given cluster.
func FindClusterIPs(client metalgo.Client, projectID, clusterID string) ([]*models.V1IPResponse, error) {
	req := &models.V1IPFindRequest{
		Projectid: projectID,
	}

	resp, err := client.IP().FindIPs(metalip.NewFindIPsParams().WithBody(req), nil)
	if err != nil {
		return nil, err
	}

	result := []*models.V1IPResponse{}
	for _, i := range resp.Payload {
		for _, t := range i.Tags {
			if tags.IsEgress(t) {
				continue
			}
			if tags.IsMachine(t) {
				continue
			}
			if tags.IsMemberOfCluster(t, clusterID) {
				result = append(result, i)
				break
			}
		}
	}

	return result, nil
}

// FindProjectIP returns the IP
func FindProjectIP(client metalgo.Client, projectID, ip string) (*models.V1IPResponse, error) {
	req := &models.V1IPFindRequest{
		Ipaddress: ip,
		Projectid: projectID,
	}

	resp, err := client.IP().FindIPs(metalip.NewFindIPsParams().WithBody(req), nil)
	if err != nil {
		return nil, err
	}

	if len(resp.Payload) != 1 {
		return nil, fmt.Errorf("ip %s is ambiguous for projectID: %s", ip, projectID)
	}

	return resp.Payload[0], nil
}

// FindProjectIPsWithTag returns the IPs of the given project that also have the given tag.
func FindProjectIPsWithTag(client metalgo.Client, projectID, tag string) ([]*models.V1IPResponse, error) {
	req := &models.V1IPFindRequest{
		Projectid: projectID,
		Tags:      []string{tag},
	}

	resp, err := client.IP().FindIPs(metalip.NewFindIPsParams().WithBody(req), nil)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
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
func FreeIP(client metalgo.Client, ip string) error {
	_, err := client.IP().FreeIP(metalip.NewFreeIPParams().WithID(ip), nil)
	if err != nil {
		return err
	}
	return nil
}

// AllocateIP acquires an IP within the given network for a given project.
func AllocateIP(client metalgo.Client, svc v1.Service, namePrefix, project, network, clusterID string) (*models.V1IPResponse, error) {
	name, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	req := &models.V1IPAllocateRequest{
		Name:      fmt.Sprintf("%s%s", namePrefix, name.String()[:5]),
		Projectid: &project,
		Networkid: &network,
		Type:      pointer.StringPtr(models.V1IPBaseTypeEphemeral),
		Tags:      []string{tags.BuildClusterServiceFQNTag(clusterID, svc.GetNamespace(), svc.GetName())},
	}

	resp, err := client.IP().AllocateIP(metalip.NewAllocateIPParams().WithBody(req), nil)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}
