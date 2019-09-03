package housekeeping

import (
	"log"
	"time"

	metalgo "github.com/metal-pod/metal-go"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/component-base/logs"

	"github.com/metal-pod/metal-ccm/pkg/controllers/loadbalancer"
)

type Housekeeper struct {
	client                *metalgo.Driver
	stop                  <-chan struct{}
	logger                *log.Logger
	k8sClient             clientset.Interface
	dynamicClient         dynamic.Interface
	ticker                *tickerSyncer
	lbController          *loadbalancer.LoadBalancerController
	lastTagSync           time.Time
	lastMetalLBConfigSync time.Time
}

// New returns a new house keeper
func New(metalClient *metalgo.Driver, stop <-chan struct{}, lbController *loadbalancer.LoadBalancerController, k8sClient clientset.Interface, dynamicClient dynamic.Interface) *Housekeeper {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm housekeeping | ")

	return &Housekeeper{
		client:        metalClient,
		stop:          stop,
		logger:        logger,
		ticker:        newTickerSyncer(logger),
		lbController:  lbController,
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
	}
}

// Run runs the housekeeper...
func (h *Housekeeper) Run() {
	h.startTagSynching()
	h.startMetalLBTriggers()
}
