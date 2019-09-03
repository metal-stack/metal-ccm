package calico

import (
	"encoding/json"
	"strings"

	"github.com/metal-pod/metal-ccm/pkg/resources/metallb"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	CalicoIPAMBlockSchema = schema.GroupVersionResource{Group: "crd.projectcalico.org", Version: "v1", Resource: "ipamblocks"}
)

type IPAMBlock struct {
	Spec IPAMBlockSpec `json:"spec"`
}

type IPAMBlocks []*IPAMBlock

type IPAMBlockSpec struct {
	Affinity string `json:"affinity"`
	Cidr     string `json:"cidr"`
}

// ListIPAMBlocks returns all ipam blocks of calico.
func ListIPAMBlocks(client dynamic.Interface) (IPAMBlocks, error) {
	resource := client.Resource(CalicoIPAMBlockSchema)

	ipamBlocksRaw, err := resource.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []*IPAMBlock
	for _, ipamBlockRaw := range ipamBlocksRaw.Items {
		raw, err := json.Marshal(ipamBlockRaw.Object)
		if err != nil {
			return nil, err
		}

		var ipamBlock IPAMBlock
		err = json.Unmarshal(raw, &ipamBlock)
		if err != nil {
			return nil, err
		}

		result = append(result, &ipamBlock)
	}

	return result, nil
}

// CidrByHost creates a mapping of the host name to the used block cidr.
func (blocks IPAMBlocks) CidrByHost() metallb.PeerAddressMap {
	res := metallb.PeerAddressMap{}
	for i, block := range blocks {
		parts := strings.SplitN(block.Spec.Affinity, ":", 2)
		if len(parts) != 2 {
			continue
		}
		res[parts[1]] = blocks[i].Spec.Cidr
	}
	return res
}
