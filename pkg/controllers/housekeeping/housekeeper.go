package housekeeping

import (
	"time"

	metalgo "github.com/metal-stack/metal-go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
)

// Housekeeper periodically updates nodes, loadbalancers and metallb
type Housekeeper struct {
	client                *metalgo.Driver
	stop                  <-chan struct{}
	k8sClient             clientset.Interface
	ticker                *tickerSyncer
	lbController          *loadbalancer.LoadBalancerController
	lastTagSync           time.Time
	lastMetalLBConfigSync time.Time
	metalAPIErrors        int32
}

// New returns a new house keeper
func New(metalClient *metalgo.Driver, stop <-chan struct{}, lbController *loadbalancer.LoadBalancerController, k8sClient clientset.Interface) *Housekeeper {
	return &Housekeeper{
		client:       metalClient,
		stop:         stop,
		ticker:       newTickerSyncer(),
		lbController: lbController,
		k8sClient:    k8sClient,
	}
}

// Run runs the housekeeper...
func (h *Housekeeper) Run() {
	h.startTagSynching()
	h.startMetalLBConfigSynching()
	h.watchNodes()
	h.runHealthCheck()
}

func (h *Housekeeper) watchNodes() {
	klog.Info("start watching nodes")
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
				klog.Info("node was added, start label syncing")
				err := h.syncMachineTagsToNodeLabels()
				if err != nil {
					klog.Errorf("synching tags failed: %v", err)
					return
				}
				klog.Info("labels synched successfully")
			},
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
				oldNode := oldObj.(*v1.Node)
				newNode := newObj.(*v1.Node)

				oldTunnelAddress, _ := loadbalancer.NodeAddress(*oldNode)
				newTunnelAddress, err := loadbalancer.NodeAddress(*newNode)
				if err != nil {
					klog.Errorf("newNode does not have a tunnelAddress, ignoring")
					return
				}
				if oldTunnelAddress == newTunnelAddress {
					// node was not modified and ip address has not changed, not updating metallb config
					return
				}

				klog.Infof("node was modified and ip address has changed, updating metallb config")

				nodes, err := kubernetes.GetNodes(h.k8sClient)
				if err != nil {
					klog.Errorf("error listing nodes: %v", err)
					return
				}
				err = h.lbController.UpdateMetalLBConfig(nodes)
				if err != nil {
					klog.Errorf("error updating metallb config: %v", err)
				}
			},
		},
	)
	go controller.Run(h.stop)
}
