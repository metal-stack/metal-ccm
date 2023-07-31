package housekeeping

import (
	"time"

	"k8s.io/klog/v2"
)

type tickerSyncer struct {
}

func newTickerSyncer() *tickerSyncer {
	return &tickerSyncer{}
}

func (s *tickerSyncer) Start(name string, period time.Duration, stopCh <-chan struct{}, fn func() error) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	// manually call to avoid initial tick delay
	if err := fn(); err != nil {
		klog.Errorf("%s failed: %v", name, err)
	}

	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				klog.Errorf("%s failed: %v", name, err)
			}
		case <-stopCh:
			return
		}
	}
}
