package loadbalancer

import (
	"reflect"
	"testing"

	"github.com/metal-stack/metal-go/api/models"
)

func TestLoadBalancerController_removeServiceTag(t *testing.T) {
	var (
		testTag1 = "cluster.metal-stack.io/id/namespace/service=6ff712b7-3087-473e-b9d2-0461c2193bdf/istio-ingress/istio-ingressgateway"
		testTag2 = "cluster.metal-stack.io/id/namespace/service=f9663b93-34bf-411e-a417-792452479d60/istio-ingress/istio-ingressgateway"
		testTag3 = "cluster.metal-stack.io/id/namespace/service=43026eb9-075c-462f-b279-f4e9f2006e03/istio/istiod"
	)

	tests := []struct {
		name       string
		ip         models.V1IPResponse
		serviceTag string
		want       []string
		wantLast   bool
	}{
		{
			name: "only own service tag",
			ip: models.V1IPResponse{
				Tags: []string{testTag1},
			},
			serviceTag: testTag1,
			want:       []string{},
			wantLast:   true,
		},
		{
			name: "own service tag and other service tag",
			ip: models.V1IPResponse{
				Tags: []string{testTag1, testTag2},
			},
			serviceTag: testTag1,
			want:       []string{testTag2},
			wantLast:   false,
		},
		{
			name: "own service tag and multiple other service tags",
			ip: models.V1IPResponse{
				Tags: []string{testTag1, testTag2, testTag3},
			},
			serviceTag: testTag1,
			want:       []string{testTag2, testTag3},
			wantLast:   false,
		},
		// unusual / erroneous cases
		{
			// in this case we allow cleanup when it's an ephemeral ip
			// this handles the case that
			name: "no service tags",
			ip: models.V1IPResponse{
				Tags: nil,
			},
			serviceTag: testTag1,
			want:       nil,
			wantLast:   true,
		},
		{
			name: "two times own service tag",
			ip: models.V1IPResponse{
				Tags: []string{testTag1, testTag1},
			},
			serviceTag: testTag1,
			want:       []string{},
			wantLast:   true,
		},
		{
			name: "only other service tag",
			ip: models.V1IPResponse{
				Tags: []string{testTag2},
			},
			serviceTag: testTag1,
			want:       []string{testTag2},
			wantLast:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LoadBalancerController{}

			got, gotLast := l.removeServiceTag(tt.ip, tt.serviceTag)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
			if gotLast != tt.wantLast {
				t.Errorf("got = %v, want %v", gotLast, tt.wantLast)
			}
		})
	}
}
