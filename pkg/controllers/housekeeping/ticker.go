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

// type resources struct {
// 	client      *metalgo.Driver
// 	kclient     kubernetes.Interface
// 	machines    *machines
// 	logger      *log.Logger
// 	partitionID string
// 	projectID   string
// }

// // newResources initializes a new resources instance.

// // kclient can only be set during the cloud.Initialize call since that is when
// // the cloud provider framework provides us with a clientset. Fortunately, the
// // initialization order guarantees that kclient won't be consumed prior to it
// // being set.
// func newResources(client *metalgo.Driver, partitionID string, projectID string) *resources {
// 	logs.InitLogs()
// 	logger := logs.NewLogger("metal-ccm resources | ")

// 	return &resources{
// 		client:      client,
// 		machines:    &machines{client: client},
// 		logger:      logger,
// 		partitionID: partitionID,
// 		projectID:   projectID,
// 	}
// }

// // ResourcesController is responsible for managing metal cloud
// // resources. It maintains a local state of the resources and
// // synchronizes when needed.
// type ResourcesController struct {
// 	kclient kubernetes.Interface

// 	resources *resources
// 	syncer    syncer

// 	metallbConfig *MetalLBConfig

// 	logger *log.Logger
// }

// // NewResourcesController returns a new resource controller.
// func NewResourcesController(r *resources, client kubernetes.Interface) *ResourcesController {
// 	r.kclient = client
// 	logger := logs.NewLogger("metal-ccm housekeeping | ")

// 	return &ResourcesController{
// 		resources:     r,
// 		kclient:       client,
// 		syncer:        newTickerSyncer(),
// 		metallbConfig: newMetalLBConfig(),
// 		logger:        logger,
// 	}
// }

// import(
// 	"time"
// )
