package housekeeping

import (
	"context"
	"fmt"
	"time"

	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
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
	resp, err := h.client.Apiv2().Health().Get(context.Background(), &apiv2.HealthServiceGetRequest{})
	if err != nil {
		h.incrementAPIErrorAndPanic()
		return err
	}

	var atleastOneServiceIsUnHealthy bool
	for _, svc := range resp.Health.Services {
		if svc.Status != apiv2.ServiceStatus_SERVICE_STATUS_HEALTHY {
			atleastOneServiceIsUnHealthy = true
		}
	}

	if atleastOneServiceIsUnHealthy {
		h.incrementAPIErrorAndPanic()
		return fmt.Errorf("metal-api is not healthy since:%d times", h.metalAPIErrors)
	}

	h.resetAPIError()
	return nil
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
