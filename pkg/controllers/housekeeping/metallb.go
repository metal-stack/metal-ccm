package housekeeping

import (
	"fmt"
	"time"

	"github.com/metal-pod/metal-ccm/pkg/resources/calico"
	"github.com/metal-pod/metal-ccm/pkg/resources/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/watch"
)

const (
	SyncMetalLBInterval         = 1 * time.Minute
	SyncMetalLBMinimalInternval = 5 * time.Second
)

func (h *Housekeeper) startMetalLBTriggers() {
	h.ticker.Start("metallb syncher", SyncMetalLBInterval, h.stop, h.updateMetalLBConfig)
	go h.watchCalico()
}

func (h *Housekeeper) updateMetalLBConfig() error {
	if time.Now().Sub(h.lastMetalLBConfigSync) < SyncMetalLBMinimalInternval {
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

func (h *Housekeeper) watchCalico() {
	ch, err := h.dynamicClient.Resource(calico.CalicoIPAMBlockSchema).Watch(metav1.ListOptions{})
	if err != nil {
		h.logger.Printf("error initializing ipam blocks watcher: %v", err)
		return
	}
	go func() {
		<-h.stop
		ch.Stop()
	}()

	h.logger.Printf("start watching ipam blocks")
	for ev := range ch.ResultChan() {
		if ev.Type == watch.Added {
			h.logger.Printf("ipam block was added, updating metallb config")
			err := h.updateMetalLBConfig()
			if err != nil {
				h.logger.Printf("error updating metallb config: %v", err)
			}
		}
	}
}
