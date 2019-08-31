package housekeeping

import (
	"log"
	"time"

	metalgo "github.com/metal-pod/metal-go"
	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
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

// New returns a new house keeper
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

// Run runs the housekeeper...
func (h *Housekeeper) Run() {
	h.runNodeWatcher()
	h.ticker.Start("tags syncer", controllerSyncTagsPeriod, h.stop, h.SyncMachineTagsToNodeLabels)
}

func (h *Housekeeper) runNodeWatcher() {
	watchlist := cache.NewListWatchFromClient(h.K8sClient.CoreV1().RESTClient(), "nodes", "", fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Node{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				h.logger.Printf("node was added, start label syncing")
				h.SyncMachineTagsToNodeLabels()
			},
		},
	)
	go controller.Run(h.stop)
}
