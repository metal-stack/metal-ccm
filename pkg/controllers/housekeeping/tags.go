package housekeeping

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	metaltag "github.com/metal-stack/metal-lib/pkg/tag"
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
	klog.Info("start syncing machine tags to node labels")

	nodes, err := kubernetes.GetNodes(context.Background(), h.k8sClient)
	if err != nil {
		return err
	}

	machineTags, err := h.getMachineTags(nodes)
	if err != nil {
		return err
	}

	updateNodeSpecBackoff := wait.Backoff{
		Steps:    20,
		Duration: 50 * time.Millisecond,
		Jitter:   1.0,
	}

	for _, n := range nodes {
		nodeName := n.Name
		tags, ok := machineTags[nodeName]
		if !ok {
			klog.Warningf("node:%s not a machine", nodeName)
			continue
		}
		labels := h.buildLabelsFromMachineTags(tags)
		err := kubernetes.UpdateNodeLabelsWithBackoff(context.Background(), h.k8sClient, n.Name, labels, updateNodeSpecBackoff)
		if err != nil {
			klog.Warningf("tags syncher failed to update tags on node:%s: %v", nodeName, err)
			continue
		}

		// check if machine has a cluster tag, if not add it
		if machineClusterTag, found := metaltag.NewTagMap(tags).Value(metaltag.ClusterID); !found || machineClusterTag != h.clusterID {
			m, err := h.ms.GetMachineFromNode(context.Background(), &n)

			if err != nil {
				klog.Warningf("unable to get machine for node:%q, not updating machine %v", n.Name, err)
				continue
			}

			if m.Allocation == nil {
				klog.Warningf("machine of node %q is not allocated, ignoring", n.Name)
				continue
			}

			err = h.ms.UpdateMachineTags(m.ID, append(tags, fmt.Sprintf("%s=%s", metaltag.ClusterID, h.clusterID)))
			if err != nil {
				klog.Warningf("unable to update machine tags of node %q, ignoring", n.Name)
				continue
			}
			klog.Infof("added cluster tag %q to machine %q", h.clusterID, *m.ID)
		}
	}
	h.lastTagSync = time.Now()

	return nil
}

// getMachineTags returns all machine tags within the shoot.
func (h *Housekeeper) getMachineTags(nodes []v1.Node) (map[string][]string, error) {
	// FIXME set context
	machines, err := h.ms.GetMachinesFromNodes(context.Background(), nodes)
	if err != nil {
		return nil, err
	}

	machineTags := make(map[string][]string)
	for _, m := range machines {
		if m.Allocation == nil {
			continue
		}
		hostname := *m.Allocation.Hostname
		machineTags[hostname] = m.Tags
	}
	return machineTags, nil
}

func (h *Housekeeper) buildLabelsFromMachineTags(tags []string) map[string]string {
	excludedLabels := map[string]bool{"networking.gardener.cloud/node-local-dns-enabled": true}

	result := make(map[string]string)

	for _, t := range tags {
		label, value, found := strings.Cut(t, "=")
		// we only add tags to the node labels that have an "="
		// and ignore labels, that are also managed by gardener
		if !found || excludedLabels[label] {
			continue
		}
		result[label] = value
	}

	return result
}
