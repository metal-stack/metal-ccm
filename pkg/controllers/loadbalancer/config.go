package loadbalancer

import (
	"context"

	"github.com/metal-stack/metal-go/api/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LoadBalancer interface {
	Namespace() string
	CalculateConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error
	WriteCRs(ctx context.Context, c client.Client) error
}
