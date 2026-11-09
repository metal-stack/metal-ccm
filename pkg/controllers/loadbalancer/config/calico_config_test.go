package config

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/tag"
	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestCalicoConfig(t *testing.T) {
	tests := []struct {
		name    string
		nws     sets.Set[string]
		ips     []*models.V1IPResponse
		nodes   []v1.Node
		wantErr error
		want    *calicoConfig
	}{
		{
			name: "one ip acquired, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: new("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
			},
			nodes:   []v1.Node{},
			wantErr: nil,
			want: &calicoConfig{
				base: &baseConfig{
					AddressPools: addressPools{
						"internet-ephemeral": {
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"84.1.1.1/32",
							},
						},
					},
					Peers: []*peer{},
				},
			},
		},
		{
			name: "two ips acquired, one static ip, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: new("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
				{
					Ipaddress: new("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
				{
					Ipaddress: new("84.1.1.3"),
					Name:      "static-ip",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("static"),
				},
			},
			nodes:   []v1.Node{},
			wantErr: nil,
			want: &calicoConfig{
				base: &baseConfig{
					AddressPools: addressPools{
						"internet-ephemeral": {
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"84.1.1.1/32",
								"84.1.1.2/32",
							},
						},
						"internet-static": {
							Name:       "internet-static",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"84.1.1.3/32",
							},
						},
					},
					Peers: []*peer{},
				},
			},
		},
		{
			name: "one ip acquired, two nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: new("2001::a:b:c"),
					Name:      "acquired-before",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
			},
			nodes: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node-a",
						Labels: map[string]string{tag.MachineNetworkPrimaryASN: "70000"},
					},
					Status: v1.NodeStatus{
						Addresses: []v1.NodeAddress{
							{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node-b",
						Labels: map[string]string{tag.MachineNetworkPrimaryASN: "70001"},
					},
					Status: v1.NodeStatus{
						Addresses: []v1.NodeAddress{
							{Type: v1.NodeInternalIP, Address: "10.0.0.2"},
						},
					},
				},
			},
			wantErr: nil,
			want: &calicoConfig{
				base: &baseConfig{
					AddressPools: addressPools{
						"internet-ephemeral": {
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"2001::a:b:c/128",
							},
						},
					},
					Peers: []*peer{
						testPeer(70000, "node-a", "10.0.0.1"),
						testPeer(70001, "node-b", "10.0.0.2"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(LoadBalancerTypeCalico, tt.ips, tt.nws, tt.nodes, nil)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("error = %v", diff)
				return
			}

			if diff := cmp.Diff(cfg, tt.want, cmpopts.IgnoreUnexported(calicoConfig{})); diff != "" {
				t.Errorf("diff = %v", diff)
			}
		})
	}
}

func TestLoadBalancerTypeFromString(t *testing.T) {
	tests := []struct {
		name    string
		lb      string
		want    LoadBalancerType
		wantErr bool
	}{
		{name: "metallb", lb: "metallb", want: LoadBalancerTypeMetalLB},
		{name: "cilium", lb: "cilium", want: LoadBalancerTypeCilium},
		{name: "calico", lb: "calico", want: LoadBalancerTypeCalico},
		{name: "default", lb: "", want: LoadBalancerTypeMetalLB},
		{name: "unknown", lb: "unknown", want: LoadBalancerType(""), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadBalancerTypeFromString(tt.lb)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff = %v", diff)
			}
		})
	}
}

// the calico types are declared for the projectcalico.org/v3 group, but the kubernetes api server
// only serves the calico custom resources in the crd.projectcalico.org/v1 group, unless the calico
// api server is deployed as well. Therefore the types must be registered with the group the custom
// resources are served in, otherwise the resources can not be written.
func TestAddCalicoToScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddCalicoToScheme(scheme); err != nil {
		t.Fatalf("AddCalicoToScheme: %v", err)
	}

	objs := []runtime.Object{
		&calicov3.BGPPeer{},
		&calicov3.BGPPeerList{},
		&calicov3.BGPConfiguration{},
		&calicov3.BGPConfigurationList{},
	}

	for _, obj := range objs {
		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			t.Fatalf("ObjectKinds(%T): %v", obj, err)
		}

		if len(gvks) != 1 {
			t.Fatalf("expected exactly one group version kind for %T, got %v", obj, gvks)
		}

		if gvks[0].Group != calicoAPIGroup || gvks[0].Version != calicoAPIVersion {
			t.Errorf("group version kind of %T = %v, want group %q version %q", obj, gvks[0], calicoAPIGroup, calicoAPIVersion)
		}
	}
}
