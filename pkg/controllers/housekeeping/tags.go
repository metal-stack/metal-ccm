package housekeeping

import (
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-stack/metal-ccm/pkg/resources/metal"
)

const (
	// SyncTagsInterval defines how often tags are synced to nodes
	SyncTagsInterval = 1 * time.Minute
	// SyncTagsMinimalInterval defines the minimal interval how often tags are synced to nodes
	SyncTagsMinimalInterval = 5 * time.Second
)

func (h *Housekeeper) startTagSynching() {
	go h.ticker.Start("tags syncher", SyncTagsInterval, h.stop, h.syncMachineTagsToNodeLabels)
}

// syncMachineTagsToNodeLabels synchronizes tags of machines in this project to labels of that node.
func (h *Housekeeper) syncMachineTagsToNodeLabels() error {
	h.logger.Println("start syncing machine tags to node labels")

	nodes, err := kubernetes.GetNodes(h.k8sClient)
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
		err := kubernetes.UpdateNodeWithBackoff(h.k8sClient, node, updateNodeSpecBackoff)
		if err != nil {
			return err
		}
	}

	h.lastTagSync = time.Now()

	return nil
}

// getMachineTags returns all machine tags within the shoot.
func (h *Housekeeper) getMachineTags(nodes []v1.Node) (map[string][]string, error) {
	machines, err := metal.GetMachinesFromNodes(h.client, nodes)
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
		parts := strings.SplitN(t, "=", 2)
		// we only add tags to the node labels that have an "="
		if len(parts) > 1 {
			result[parts[0]] = strings.Join(parts[1:], "=")
		}
	}
	return result
}
