package loadbalancer

import (
	"reflect"
	"testing"

	"github.com/metal-stack/metal-go/api/models"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestFixedIPsFromService(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		specIP      string
		want        []string
	}{
		{
			name: "no annotation and no spec ip",
			want: nil,
		},
		{
			name:        "single ip from annotation",
			annotations: map[string]string{"metallb.io/loadBalancerIPs": "10.100.0.1"},
			want:        []string{"10.100.0.1"},
		},
		{
			name:        "multiple comma-separated ips from annotation",
			annotations: map[string]string{"metallb.io/loadBalancerIPs": "10.100.0.1,10.100.0.2"},
			want:        []string{"10.100.0.1", "10.100.0.2"},
		},
		{
			name:        "ips with surrounding whitespace and trailing comma",
			annotations: map[string]string{"metallb.io/loadBalancerIPs": " 10.100.0.1 , 10.100.0.2 ,"},
			want:        []string{"10.100.0.1", "10.100.0.2"},
		},
		{
			name:        "dual stack ipv4 and ipv6",
			annotations: map[string]string{"metallb.io/loadBalancerIPs": "10.100.0.1,2001:db8::1"},
			want:        []string{"10.100.0.1", "2001:db8::1"},
		},
		{
			name:   "fallback to deprecated spec.loadBalancerIP",
			specIP: "10.100.0.1",
			want:   []string{"10.100.0.1"},
		},
		{
			name:        "annotation takes precedence over deprecated spec.loadBalancerIP",
			annotations: map[string]string{"metallb.io/loadBalancerIPs": "10.100.0.1"},
			specIP:      "10.100.0.2",
			want:        []string{"10.100.0.1"},
		},
		{
			name:        "whitespace only annotation falls back to spec ip",
			annotations: map[string]string{"metallb.io/loadBalancerIPs": "   "},
			specIP:      "10.100.0.1",
			want:        []string{"10.100.0.1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
				Spec: v1.ServiceSpec{
					LoadBalancerIP: tt.specIP,
				},
			}

			got := fixedIPsFromService(service)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddressPoolFromService(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        string
		wantOK      bool
	}{
		{
			name:   "no annotation",
			wantOK: false,
		},
		{
			name:        "new annotation",
			annotations: map[string]string{"metallb.io/address-pool": "network-a"},
			want:        "network-a",
			wantOK:      true,
		},
		{
			name:        "deprecated annotation is honored for backwards compatibility",
			annotations: map[string]string{"metallb.universe.tf/address-pool": "network-a"},
			want:        "network-a",
			wantOK:      true,
		},
		{
			name: "new annotation takes precedence over deprecated one",
			annotations: map[string]string{
				"metallb.io/address-pool":          "network-new",
				"metallb.universe.tf/address-pool": "network-old",
			},
			want:   "network-new",
			wantOK: true,
		},
		{
			name: "empty new annotation falls back to deprecated one",
			annotations: map[string]string{
				"metallb.io/address-pool":          "",
				"metallb.universe.tf/address-pool": "network-old",
			},
			want:   "network-old",
			wantOK: true,
		},
		{
			name:        "empty new annotation and no deprecated one",
			annotations: map[string]string{"metallb.io/address-pool": ""},
			wantOK:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			got, gotOK := addressPoolFromService(service)
			if got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
			if gotOK != tt.wantOK {
				t.Errorf("gotOK = %v, want %v", gotOK, tt.wantOK)
			}
		})
	}
}
