package housekeeping

import (
	"context"
	"time"

	"slices"

	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
)

const (
	// SyncTagsInterval defines how often ssh public keys are synched to metal machines
	syncSSHKeysInterval = 5 * time.Minute
)

func (h *Housekeeper) startSSHKeysSynching() {
	if len(h.sshPublicKey) == 0 {
		klog.Warningf("ssh public keys not set, not synching back to machines")
		return
	}

	go h.ticker.Start("ssh public keys syncher", syncSSHKeysInterval, h.stop, h.syncSSHKeys)
}

// syncSSHKeys synchronizes ssh public keys to machines.
func (h *Housekeeper) syncSSHKeys() error {
	klog.Info("start syncing ssh public keys to machine")

	nodes, err := kubernetes.GetNodes(h.k8sClient)
	if err != nil {
		return err
	}

	for _, n := range nodes {
		n := n
		m, err := h.ms.GetMachineFromNode(context.Background(), &n)

		if err != nil {
			klog.Warningf("unable to get machine for node:%q, not updating machine %v", n.Name, err)
			continue
		}

		if m.Allocation == nil {
			klog.Warningf("machine of node %q is not allocated, ignoring", n.Name)
			continue
		}

		if slices.Contains(m.Allocation.SSHPubKeys, h.sshPublicKey) {
			klog.Infof("machine %q has already actual ssh public keys", *m.Allocation.Hostname)
			continue
		}

		_, err = h.client.Machine().UpdateMachine(machine.NewUpdateMachineParams().WithBody(&models.V1MachineUpdateRequest{
			ID:         m.ID,
			SSHPubKeys: []string{h.sshPublicKey},
		}), nil)
		if err != nil {
			klog.Errorf("unable to update ssh public keys for machine %q %v", *m.Allocation.Hostname, err)
			continue
		}
	}

	return nil
}
