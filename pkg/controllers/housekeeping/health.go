package housekeeping

import (
	"fmt"
	"time"

	"github.com/metal-stack/metal-lib/pkg/healthstatus"
	"k8s.io/klog/v2"
)

const (
	healthInterval = 1 * time.Minute
	maxAPIErrors   = 60
)

func (h *Housekeeper) runHealthCheck() {
	go h.ticker.Start("metal-api healthcheck", healthInterval, h.stop, h.checkMetalAPIHealth)
}

func (h *Housekeeper) checkMetalAPIHealth() error {
	klog.Infof("checking metal-api health, total errors:%d", h.metalAPIErrors)
	resp, err := h.client.Health().Health(nil, nil)
	if err != nil {
		h.incrementAPIErrorAndPanic()
		return err
	}

	if resp.Payload != nil && resp.Payload.Status != nil && *resp.Payload.Status == string(healthstatus.HealthStatusHealthy) {
		h.resetAPIError()
		return nil
	}
	h.incrementAPIErrorAndPanic()
	return fmt.Errorf("metal-api is not healthy since:%d times", h.metalAPIErrors)
}

func (h *Housekeeper) incrementAPIErrorAndPanic() {
	h.metalAPIErrors++
	if h.metalAPIErrors > maxAPIErrors {
		klog.Fatalf("metal-api was not healthy for more than:%d times, panic", maxAPIErrors)
	}
}

func (h *Housekeeper) resetAPIError() {
	h.metalAPIErrors = 0
}
