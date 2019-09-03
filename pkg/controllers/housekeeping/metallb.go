package housekeeping

import (
	"fmt"
	"time"

	"github.com/metal-pod/metal-ccm/pkg/resources/kubernetes"
)

const (
	SyncMetalLBInterval         = 1 * time.Minute
	SyncMetalLBMinimalInternval = 5 * time.Second
)

func (h *Housekeeper) startMetalLBConfigSynching() {
	go h.ticker.Start("metallb syncher", SyncMetalLBInterval, h.stop, h.updateMetalLBConfig)
}

func (h *Housekeeper) updateMetalLBConfig() error {
	if time.Since(h.lastMetalLBConfigSync) < SyncMetalLBMinimalInternval {
		return nil
	}
	nodes, err := kubernetes.GetNodes(h.k8sClient)
	if err != nil {
		return fmt.Errorf("error listing nodes: %v", err)
	}
	err = h.lbController.UpdateMetalLBConfig(nodes)
	if err != nil {
		return fmt.Errorf("error updating metallb config: %v", err)
	}
	h.lastMetalLBConfigSync = time.Now()
	return nil
}
