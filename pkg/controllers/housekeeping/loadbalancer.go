package housekeeping

import (
	"context"
	"fmt"
	"time"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
)

const (
	syncLoadBalancerInterval         = 1 * time.Minute
	syncLoadBalancerMinimalInternval = 5 * time.Second
)

func (h *Housekeeper) startLoadBalancerConfigSynching() {
	go h.ticker.Start("load balancer syncher", syncLoadBalancerInterval, h.stop, h.updateLoadBalancerConfig)
}

func (h *Housekeeper) updateLoadBalancerConfig() error {
	if time.Since(h.lastLoadBalancerConfigSync) < syncLoadBalancerMinimalInternval {
		return nil
	}
	nodes, err := kubernetes.GetNodes(context.Background(), h.k8sClient)
	if err != nil {
		return fmt.Errorf("error listing nodes: %w", err)
	}
	err = h.lbController.UpdateLoadBalancerConfig(context.Background(), nodes)
	if err != nil {
		return fmt.Errorf("error updating load balancer config: %w", err)
	}
	h.lastLoadBalancerConfigSync = time.Now()
	return nil
}
