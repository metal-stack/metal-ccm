package housekeeping

import (
	"context"
	"time"

	metalgo "github.com/metal-stack/metal-go"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-stack/metal-ccm/pkg/resources/metal"
)

// Housekeeper periodically updates nodes and load balancers
type Housekeeper struct {
	client                     metalgo.Client
	stop                       <-chan struct{}
	k8sClient                  clientset.Interface
	ticker                     *tickerSyncer
	lbController               *loadbalancer.LoadBalancerController
	lastTagSync                time.Time
	lastLoadBalancerConfigSync time.Time
	metalAPIErrors             int32
	ms                         *metal.MetalService
	sshPublicKey               string
	clusterID                  string
}

// New returns a new house keeper
func New(metalClient metalgo.Client, stop <-chan struct{}, lbController *loadbalancer.LoadBalancerController, k8sClient clientset.Interface, projectID string, sshPublicKey string, clusterID string) *Housekeeper {
	return &Housekeeper{
		client:       metalClient,
		stop:         stop,
		ticker:       newTickerSyncer(),
		lbController: lbController,
		k8sClient:    k8sClient,
		ms:           metal.New(metalClient, k8sClient, projectID),
		sshPublicKey: sshPublicKey,
		clusterID:    clusterID,
	}
}

// Run runs the housekeeper...
func (h *Housekeeper) Run() error {
	h.startTagSynching()
	h.startLoadBalancerConfigSynching()
	h.startSSHKeysSynching()
	err := h.watchNodes()
	if err != nil {
		return err
	}
	h.runHealthCheck()
	return nil
}

func (h *Housekeeper) watchNodes() error {
	klog.Info("start watching nodes")

	informerFactory := informers.NewSharedInformerFactory(h.k8sClient, time.Second*30)
	nodeInformer := informerFactory.Core().V1().Nodes()
	_, err := nodeInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
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
			UpdateFunc: func(oldObj any, newObj any) {
				oldNode := oldObj.(*v1.Node)
				newNode := newObj.(*v1.Node)

				oldTunnelAddress, _ := kubernetes.NodeAddress(*oldNode)
				newTunnelAddress, err := kubernetes.NodeAddress(*newNode)
				if err != nil {
					klog.Error("newNode does not have a tunnelAddress, ignoring")
					return
				}
				if oldTunnelAddress == newTunnelAddress {
					// node was not modified and ip address has not changed, not updating load balancer config
					return
				}

				klog.Info("node was modified and ip address has changed, updating load balancer config")

				nodes, err := kubernetes.GetNodes(context.Background(), h.k8sClient)
				if err != nil {
					klog.Errorf("error listing nodes: %v", err)
					return
				}
				err = h.lbController.UpdateLoadBalancerConfig(context.Background(), nodes)
				if err != nil {
					klog.Errorf("error updating load balancer config: %v", err)
				}
			},
		},
	)
	if err != nil {
		return err
	}
	informerFactory.Start(wait.NeverStop)
	go informerFactory.WaitForCacheSync(wait.NeverStop)
	return nil
}
