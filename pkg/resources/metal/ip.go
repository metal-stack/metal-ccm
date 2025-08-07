package metal

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/metal-stack/metal-ccm/pkg/tags"

	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"

	v1 "k8s.io/api/core/v1"

	"github.com/google/uuid"
)

// FindClusterIPs returns the allowed IPs of the given cluster.
func (ms *MetalService) FindClusterIPs(ctx context.Context, clusterID string) ([]*apiv2.IP, error) {
	resp, err := ms.client.Apiv2().IP().List(ctx, connect.NewRequest(&apiv2.IPServiceListRequest{Project: ms.project}))
	if err != nil {
		return nil, err
	}

	result := []*apiv2.IP{}
	for _, i := range resp.Msg.Ips {
		if i.Meta == nil || i.Meta.Labels == nil || i.Meta.Labels.Labels == nil {
			continue
		}

		if _, ok := i.Meta.Labels.Labels[tag.ClusterEgress]; ok {
			continue
		}

		if _, ok := i.Meta.Labels.Labels[tag.MachineID]; ok {
			continue
		}

		for _, t := range i.Meta.Labels.Labels {
			if tags.IsMemberOfCluster(t, clusterID) {
				result = append(result, i)
				break
			}
		}
	}

	return result, nil
}

// FindProjectIP returns the IP
func (ms *MetalService) FindProjectIP(ctx context.Context, ip string) (*apiv2.IP, error) {
	resp, err := ms.client.Apiv2().IP().Get(ctx, connect.NewRequest(&apiv2.IPServiceGetRequest{
		Project: ms.project,
		Ip:      ip,
	}))
	if err != nil {
		return nil, err
	}

	return resp.Msg.Ip, nil
}

// FindProjectIPsWithTag returns the IPs of the given project that also have the given tag.
func (ms *MetalService) FindProjectIPsWithTag(ctx context.Context, tagString string) ([]*apiv2.IP, error) {

	tagMap := tag.NewTagMap([]string{tagString})

	req := &apiv2.IPServiceListRequest{
		Project: ms.project,
		Query: &apiv2.IPQuery{
			Labels: &apiv2.Labels{
				Labels: tagMap,
			},
		},
	}

	resp, err := ms.client.Apiv2().IP().List(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	return resp.Msg.Ips, nil
}

// FreeIP frees the given IP address.
func (ms *MetalService) FreeIP(ctx context.Context, ip string) error {
	_, err := ms.client.Apiv2().IP().Delete(ctx, connect.NewRequest(&apiv2.IPServiceDeleteRequest{Ip: ip, Project: ms.project}))
	if err != nil {
		return err
	}
	return nil
}

// AllocateIP acquires an IP within the given network for a given project.
func (ms *MetalService) AllocateIP(ctx context.Context, svc v1.Service, namePrefix, network, clusterID string) (*apiv2.IP, error) {
	name, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	req := &apiv2.IPServiceCreateRequest{
		Name:    pointer.Pointer(fmt.Sprintf("%s%s", namePrefix, name.String()[:5])),
		Project: ms.project,
		Network: network,
		Type:    apiv2.IPType_IP_TYPE_EPHEMERAL.Enum(),
		Labels: &apiv2.Labels{
			Labels: tags.BuildClusterServiceFQNLabel(clusterID, svc.GetNamespace(), svc.GetName()),
		},
	}

	resp, err := ms.client.Apiv2().IP().Create(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	return resp.Msg.Ip, nil
}

// UpdateIP updates the given IP address.
func (ms *MetalService) UpdateIP(ctx context.Context, body *apiv2.IPServiceUpdateRequest) (*apiv2.IP, error) {
	resp, err := ms.client.Apiv2().IP().Update(ctx, connect.NewRequest(body))
	if err != nil {
		return nil, err
	}

	return resp.Msg.Ip, nil
}
