package housekeeping

import (
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/metal-pod/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-pod/metal-ccm/pkg/resources/metal"
)

// SyncMachineTagsToNodeLabels synchronizes tags of machines in this project to labels of that node.
func (h *Housekeeper) SyncMachineTagsToNodeLabels() error {
	h.logger.Println("Start syncing machine tags to node labels")

	nodes, err := kubernetes.GetNodes(h.K8sClient)
	if err != nil {
		return err
	}

	machineTags, err := h.getMachineTags(nodes)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		nodeName := n.Name
		tags, ok := machineTags[nodeName]
		if !ok {
			h.logger.Printf("warning: node:%s not a machine", nodeName)
			continue
		}
		labels := h.buildLabelsFromMachineTags(tags)
		h.logger.Printf("ensuring node tags of %q: %v", nodeName, labels)

		for key, value := range labels {
			n.Labels[key] = value
		}
	}

	updateNodeSpecBackoff := wait.Backoff{
		Steps:    20,
		Duration: 50 * time.Millisecond,
		Jitter:   1.0,
	}
	for _, node := range nodes {
		err := kubernetes.UpdateNodeWithBackoff(h.K8sClient, &node, updateNodeSpecBackoff)
		if err != nil {
			return err
		}
	}

	return nil
}

// getMachineTags returns all machine tags within the shoot.
func (h *Housekeeper) getMachineTags(nodes []v1.Node) (map[string][]string, error) {
	machines, err := metal.GetMachinesFromNodes(h.client, nodes...)
	if err != nil {
		return nil, err
	}

	machineTags := make(map[string][]string)
	for _, m := range machines {
		hostname := *m.Allocation.Hostname
		machineTags[hostname] = m.Tags
	}
	return machineTags, nil
}

func (h *Housekeeper) buildLabelsFromMachineTags(tags []string) map[string]string {
	result := make(map[string]string)
	for _, t := range tags {
		parts := strings.Split(t, "=")
		// TODO labels must have a value ?
		// if len(parts) == 0 {
		// 	result[t] = ""
		// }
		// if len(parts) == 1 {
		// 	result[parts[0]] = ""
		// }
		if len(parts) > 1 {
			result[parts[0]] = strings.Join(parts[1:], "=")
		}
	}
	return result
}
