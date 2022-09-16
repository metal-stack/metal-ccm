package housekeeping

import (
	"context"
	"fmt"
	"time"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
)

const (
	syncMetalLBInterval         = 1 * time.Minute
	syncMetalLBMinimalInternval = 5 * time.Second
)

func (h *Housekeeper) startMetalLBConfigSynching() {
	go h.ticker.Start("metallb syncher", syncMetalLBInterval, h.stop, h.updateMetalLBConfig)
}

func (h *Housekeeper) updateMetalLBConfig() error {
	if time.Since(h.lastMetalLBConfigSync) < syncMetalLBMinimalInternval {
		return nil
	}
	nodes, err := kubernetes.GetNodes(h.k8sClient)
	if err != nil {
		return fmt.Errorf("error listing nodes: %w", err)
	}
	err = h.lbController.UpdateMetalLBConfig(context.Background(), nodes)
	if err != nil {
		return fmt.Errorf("error updating metallb config: %w", err)
	}
	h.lastMetalLBConfigSync = time.Now()
	return nil
}
