package housekeeping

import (
	"fmt"
	"net/http"
	"time"

	"github.com/metal-stack/metal-lib/pkg/healthstatus"
	"k8s.io/klog/v2"
)

const (
	probeAddr         = ":10268"
	probeReadTimeout  = 5 * time.Second
	probeWriteTimeout = 5 * time.Second
)

// Run probe endpoint outside of controller runtime, as there is no compact way
// to share the metal-client to a second controller
// CCM restart on api failure is expected to be configured in the deployment
func (h *Housekeeper) startProbeServer() {
	mux := http.NewServeMux()

	metalAPICheckHandler := func(w http.ResponseWriter, _ *http.Request) {
		if err := h.checkMetalAPI(); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			klog.Error(err)
		}
	}

	mux.HandleFunc("/healthz", metalAPICheckHandler)
	mux.HandleFunc("/readyz", metalAPICheckHandler)

	server := &http.Server{
		Addr:              probeAddr,
		Handler:           mux,
		ReadHeaderTimeout: probeReadTimeout,
		WriteTimeout:      probeWriteTimeout,
	}

	go func() {
		klog.Infof("starting probe server on %s", probeAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("probe server terminated: %v", err)
		}
	}()
}

func (h *Housekeeper) checkMetalAPI() error {
	klog.Infof("checking metal-api health")
	resp, err := h.client.Health().Health(nil, nil)
	if err != nil {
		klog.Errorf("metal-api health check failed: %v", err)
		return err
	}

	if resp.Payload != nil && resp.Payload.Status != nil && *resp.Payload.Status == string(healthstatus.HealthStatusHealthy) {
		return nil
	}

	err = fmt.Errorf("metal-api is not healthy with response %v: %v", *resp.Payload.Status, resp.Payload.Message)
	klog.Error(err)
	return err
}
