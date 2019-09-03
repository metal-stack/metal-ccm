package housekeeping

import (
	"log"
	"time"
)

type tickerSyncer struct {
	logger *log.Logger
}

func newTickerSyncer(logger *log.Logger) *tickerSyncer {
	return &tickerSyncer{
		logger: logger,
	}
}

func (s *tickerSyncer) Start(name string, period time.Duration, stopCh <-chan struct{}, fn func() error) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	// manually call to avoid initial tick delay
	if err := fn(); err != nil {
		s.logger.Printf("%s failed: %s", name, err)
	}

	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				s.logger.Printf("%s failed: %s", name, err)
			}
		case <-stopCh:
			return
		}
	}
}
