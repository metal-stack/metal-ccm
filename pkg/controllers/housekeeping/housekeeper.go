package housekeeping

import (
	"log"
	"time"

	metalgo "github.com/metal-stack/metal-go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/logs"

	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
)

type Housekeeper struct {
	client                *metalgo.Driver
	stop                  <-chan struct{}
	logger                *log.Logger
	k8sClient             clientset.Interface
	ticker                *tickerSyncer
	lbController          *loadbalancer.LoadBalancerController
	lastTagSync           time.Time
	lastMetalLBConfigSync time.Time
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
	h.startTagSynching()
	h.startMetalLBConfigSynching()
	h.watchNodes()
}

func (h *Housekeeper) watchNodes() {
	h.logger.Printf("start watching nodes")
	watchlist := cache.NewListWatchFromClient(h.k8sClient.CoreV1().RESTClient(), "nodes", "", fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Node{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if time.Since(h.lastTagSync) < SyncTagsMinimalInterval {
					return
				}
				h.logger.Printf("node was added, start label syncing")
				err := h.syncMachineTagsToNodeLabels()
				if err != nil {
					h.logger.Printf("synching tags failed: %v", err)
					return
				} else {
					h.logger.Printf("labels synched successfully")
				}
			},
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
				oldNode := oldObj.(*v1.Node)
				newNode := newObj.(*v1.Node)

				oldTunnelAddress, _ := loadbalancer.CalicoTunnelAddress(*oldNode)
				newTunnelAddress, err := loadbalancer.CalicoTunnelAddress(*newNode)
				if err != nil {
					h.logger.Printf("newNode does not have a tunnelAddress, ignoring")
					return
				}
				if oldTunnelAddress == newTunnelAddress {
					h.logger.Printf("node was not modified and calico tunnel address has not changed, not updating metallb config")
					return
				}

				h.logger.Printf("node was modified and calico tunnel address has changed, updating metallb config")

				nodes, err := kubernetes.GetNodes(h.k8sClient)
				if err != nil {
					h.logger.Printf("error listing nodes: %v", err)
					return
				}
				err = h.lbController.UpdateMetalLBConfig(nodes)
				if err != nil {
					h.logger.Printf("error updating metallb config: %v", err)
				}
			},
		},
	)
	go controller.Run(h.stop)
}
