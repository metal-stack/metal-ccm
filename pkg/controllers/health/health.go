package health

import (
	"fmt"
	"net/http"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/healthstatus"
	"k8s.io/controller-manager/pkg/healthz"
	"k8s.io/klog/v2"
)

// HealthCheck is a controller that exposes metal-api health via the CCM /healthz endpoint.
type HealthCheck struct {
	client metalgo.Client
}

func New(client metalgo.Client) *HealthCheck {
	return &HealthCheck{client: client}
}

func (h *HealthCheck) Name() string {
	return "metal-api-health"
}

func (h *HealthCheck) HealthChecker() healthz.UnnamedHealthChecker {
	return h
}

func (h *HealthCheck) Check(_ *http.Request) error {
	resp, err := h.client.Health().Health(nil, nil)
	if err != nil {
		return fmt.Errorf("metal-api health endpoint not reachable: %w", err)
	}

	if resp.Payload == nil || resp.Payload.Status == nil || *resp.Payload.Status != string(healthstatus.HealthStatusHealthy) {
		return fmt.Errorf("metal-api not healthy")
	}

	klog.Info("external health check: metal-api is healthy")

	return nil
}
