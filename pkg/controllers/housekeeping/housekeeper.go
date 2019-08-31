package housekeeping

import (
	"log"
	"time"

	metalgo "github.com/metal-pod/metal-go"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/logs"
)

const (
	controllerSyncTagsPeriod = 1 * time.Minute
)

type Housekeeper struct {
	client    *metalgo.Driver
	stop      <-chan struct{}
	logger    *log.Logger
	K8sClient clientset.Interface
	ticker    *tickerSyncer
}

func New(metalClient *metalgo.Driver, stop <-chan struct{}) *Housekeeper {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm housekeeping | ")

	return &Housekeeper{
		client: metalClient,
		stop:   stop,
		logger: logger,
		ticker: newTickerSyncer(logger),
	}
}

func (h *Housekeeper) Run() {
	h.ticker.Start("tags syncer", controllerSyncTagsPeriod, h.stop, h.SyncMachineTagsToNodeLabels)
}
