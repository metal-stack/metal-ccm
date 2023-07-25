package metal

import (
	"context"
	"fmt"

	"github.com/metal-stack/metal-ccm/pkg/tags"

	metalip "github.com/metal-stack/metal-go/api/client/ip"

	"github.com/metal-stack/metal-go/api/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/google/uuid"
)

// FindClusterIPs returns the allowed IPs of the given cluster.
func (ms *MetalService) FindClusterIPs(ctx context.Context, projectID, clusterID string) ([]*models.V1IPResponse, error) {
	req := &models.V1IPFindRequest{
		Projectid: projectID,
	}

	resp, err := ms.client.IP().FindIPs(metalip.NewFindIPsParams().WithBody(req).WithContext(ctx), nil)
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
func (ms *MetalService) FindProjectIP(ctx context.Context, projectID, ip string) (*models.V1IPResponse, error) {
	req := &models.V1IPFindRequest{
		Ipaddress: ip,
		Projectid: projectID,
	}

	resp, err := ms.client.IP().FindIPs(metalip.NewFindIPsParams().WithBody(req).WithContext(ctx), nil)
	if err != nil {
		return nil, err
	}

	if len(resp.Payload) != 1 {
		return nil, fmt.Errorf("ip %s is ambiguous for projectID: %s", ip, projectID)
	}

	return resp.Payload[0], nil
}

// FindProjectIPsWithTag returns the IPs of the given project that also have the given tag.
func (ms *MetalService) FindProjectIPsWithTag(ctx context.Context, projectID, tag string) ([]*models.V1IPResponse, error) {
	req := &models.V1IPFindRequest{
		Projectid: projectID,
		Tags:      []string{tag},
	}

	resp, err := ms.client.IP().FindIPs(metalip.NewFindIPsParams().WithBody(req).WithContext(ctx), nil)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// FreeIP frees the given IP address.
func (ms *MetalService) FreeIP(ctx context.Context, ip string) error {
	_, err := ms.client.IP().FreeIP(metalip.NewFreeIPParams().WithID(ip).WithContext(ctx), nil)
	if err != nil {
		return err
	}
	return nil
}

// AllocateIP acquires an IP within the given network for a given project.
func (ms *MetalService) AllocateIP(ctx context.Context, svc v1.Service, namePrefix, project, network, clusterID string) (*models.V1IPResponse, error) {
	name, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	req := &models.V1IPAllocateRequest{
		Name:      fmt.Sprintf("%s%s", namePrefix, name.String()[:5]),
		Projectid: &project,
		Networkid: &network,
		Type:      pointer.String(models.V1IPBaseTypeEphemeral),
		Tags:      []string{tags.BuildClusterServiceFQNTag(clusterID, svc.GetNamespace(), svc.GetName())},
	}

	resp, err := ms.client.IP().AllocateIP(metalip.NewAllocateIPParams().WithBody(req).WithContext(ctx), nil)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

// UpdateIP updates the given IP address.
func (ms *MetalService) UpdateIP(ctx context.Context, body *models.V1IPUpdateRequest) (*models.V1IPResponse, error) {
	resp, err := ms.client.IP().UpdateIP(metalip.NewUpdateIPParams().WithBody(body).WithContext(ctx), nil)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}
