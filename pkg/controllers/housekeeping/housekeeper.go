package housekeeping

import (
	"log"
	"time"

	metalgo "github.com/metal-pod/metal-go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/logs"

	"github.com/metal-pod/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-pod/metal-ccm/pkg/resources/kubernetes"
)

const (
	controllerSyncTagsPeriod = 1 * time.Minute
)

type Housekeeper struct {
	client       *metalgo.Driver
	stop         <-chan struct{}
	logger       *log.Logger
	k8sClient    clientset.Interface
	ticker       *tickerSyncer
	lbController *loadbalancer.LoadBalancerController
}

// New returns a new house keeper
func New(metalClient *metalgo.Driver, stop <-chan struct{}, lbController *loadbalancer.LoadBalancerController, k8sClient clientset.Interface) *Housekeeper {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm housekeeping | ")

	return &Housekeeper{
		client:       metalClient,
		stop:         stop,
		logger:       logger,
		ticker:       newTickerSyncer(logger),
		lbController: lbController,
		k8sClient:    k8sClient,
	}
}

// Run runs the housekeeper...
func (h *Housekeeper) Run() {
	h.runNodeWatcher()
	h.ticker.Start("tags syncher", controllerSyncTagsPeriod, h.stop, h.SyncMachineTagsToNodeLabels)
}

func (h *Housekeeper) runNodeWatcher() {
	watchlist := cache.NewListWatchFromClient(h.k8sClient.CoreV1().RESTClient(), "nodes", "", fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Node{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				h.logger.Printf("node was added, start label syncing and updating metallb config")
				h.SyncMachineTagsToNodeLabels()

				nodes, err := kubernetes.GetNodes(h.k8sClient)
				if err != nil {
					h.logger.Printf("unable to fetch nodes:%v", err)
				} else {

					h.lbController.UpdateMetalLBConfig(nodes)
				}
			},
		},
	)
	go controller.Run(h.stop)
}
