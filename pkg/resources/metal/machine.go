package metal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/metal-stack/metal-ccm/pkg/resources/constants"
	clientset "k8s.io/client-go/kubernetes"

	metalclient "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"

	"github.com/metal-stack/metal-lib/pkg/cache"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetalService struct {
	client                 metalclient.Client
	k8sclient              clientset.Interface
	machineByUUIDCache     *cache.Cache[string, *apiv2.Machine]
	machineByHostnameCache *cache.Cache[string, *apiv2.Machine]
	project                string
}

func New(client metalclient.Client, k8sclient clientset.Interface, projectID string) *MetalService {
	machineByUUIDCache := cache.New(time.Minute, func(ctx context.Context, id string) (*apiv2.Machine, error) {
		resp, err := client.Apiv2().Machine().Get(ctx, connect.NewRequest(&apiv2.MachineServiceGetRequest{
			Uuid:    id,
			Project: projectID,
		}))
		if err != nil {
			return nil, err
		}

		if resp.Msg.Machine.Allocation == nil {
			return nil, fmt.Errorf("machine %q is not allocated", id)
		}
		if resp.Msg.Machine.Allocation.Project != projectID {
			return nil, fmt.Errorf("machine %q is allocated in the wrong project: %q", id, projectID)
		}

		return resp.Msg.Machine, nil
	})
	machineByHostnameCache := cache.New(time.Minute, func(ctx context.Context, hostname string) (*apiv2.Machine, error) {
		resp, err := client.Apiv2().Machine().List(ctx, connect.NewRequest(&apiv2.MachineServiceListRequest{
			Project: projectID,
			Query: &apiv2.MachineQuery{
				Allocation: &apiv2.MachineAllocationQuery{
					Hostname: &hostname,
					Project:  &projectID,
				},
			},
		}))
		if err != nil {
			return nil, err
		}
		if len(resp.Msg.Machines) != 1 {
			return nil, fmt.Errorf("not exactly one machine was found for hostname:%q", hostname)
		}
		return resp.Msg.Machines[0], nil
	})
	ms := &MetalService{
		client:                 client,
		k8sclient:              k8sclient,
		machineByUUIDCache:     machineByUUIDCache,
		machineByHostnameCache: machineByHostnameCache,
		project:                projectID,
	}
	return ms
}

// GetMachinesFromNodes gets metal machines from K8s nodes.
func (ms *MetalService) GetMachinesFromNodes(ctx context.Context, nodes []v1.Node) ([]*apiv2.Machine, error) {
	var mm []*apiv2.Machine
	for _, n := range nodes {
		m, err := ms.GetMachineFromProviderID(ctx, n.Spec.ProviderID)
		if err != nil {
			return nil, err
		}
		mm = append(mm, m)
	}

	return mm, nil
}

// GetMachineFromNodeName returns a machine where hostname matches the kubernetes node.Name.
func (ms *MetalService) GetMachineFromNodeName(ctx context.Context, nodeName types.NodeName) (*apiv2.Machine, error) {
	node, err := ms.k8sclient.CoreV1().Nodes().Get(ctx, string(nodeName), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return ms.GetMachineFromProviderID(ctx, node.Spec.ProviderID)
}

// GetMachineFromProviderID uses providerID to get the machine id and returns the machine.
func (ms *MetalService) GetMachineFromProviderID(ctx context.Context, providerID string) (*apiv2.Machine, error) {
	id, err := decodeMachineIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	machine, err := ms.machineByUUIDCache.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return machine, nil
}

// GetMachineFromNode try providerID first, otherwise node.Name to fetch the machine.
func (ms *MetalService) GetMachineFromNode(ctx context.Context, node *v1.Node) (*apiv2.Machine, error) {
	var (
		machine *apiv2.Machine
		err     error
	)
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}
	// ProviderID is empty during the machine provisioning period and will be set a bit later
	// from the InstanceID call against the cloud-controller-manager
	// so as long as providerID is empty make an expensive machine find request,
	// but after it is set, we switch to a simple machine get request.
	if strings.HasPrefix(node.Spec.ProviderID, constants.ProviderName+"://") {
		var id string
		id, err = decodeMachineIDFromProviderID(node.Spec.ProviderID)
		if err != nil {
			return nil, err
		}
		machine, err = ms.machineByUUIDCache.Get(ctx, id)
	} else {
		machine, err = ms.machineByHostnameCache.Get(ctx, node.Name)
	}
	return machine, err
}

// UpdateMachineTags sets the machine tags.
func (ms *MetalService) UpdateMachineTags(ctx context.Context, m string, tags []string) error {
	_, err := ms.client.Apiv2().Machine().Update(ctx, connect.NewRequest(&apiv2.MachineServiceUpdateRequest{
		Uuid: m,
		Tags: tags,
	}))
	if err != nil {
		return err
	}
	return nil
}

// machineIDFromProviderID returns a machine's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: metal://partition-id/machine-id.
func decodeMachineIDFromProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", errors.New("providerID cannot be empty")
	}

	if !strings.HasPrefix(providerID, constants.ProviderName+"://") {
		return "", fmt.Errorf("unexpected providerID format %q, format should be %q or %q", providerID, constants.ProviderName+"://<machine-id>", constants.ProviderName+"://<partition-id>/<machine-id>")
	}

	idparts := strings.Split(providerID, "/")
	return idparts[len(idparts)-1], nil
}
