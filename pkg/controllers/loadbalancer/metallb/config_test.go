package metallb

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	testNetworks = sets.New(
		"internet",
		"shared-storage-network",
		"mpls-network",
		"dmz-network",
	)
)

func TestMetalLBConfig_CalculateConfig(t *testing.T) {
	tests := []struct {
		name    string
		nws     sets.Set[string]
		ips     []*models.V1IPResponse
		nodes   []v1.Node
		wantErr error
		want    *metalLBConfig
	}{
		{
			name: "one ip acquired, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.Pointer("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
			},
			nodes:   []v1.Node{},
			wantErr: nil,
			want: &metalLBConfig{
				Config: loadbalancer.Config{
					AddressPools: []*loadbalancer.AddressPool{
						{
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs:      []string{"84.1.1.1/32"},
						},
					},
				},
				Peers:     []*loadbalancer.Peer{},
				namespace: metallbNamespace,
			},
		},
		{
			name: "two ips acquired, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.Pointer("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
				{
					Ipaddress: pointer.Pointer("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
			},
			nodes:   []v1.Node{},
			wantErr: nil,
			want: &metalLBConfig{
				Config: loadbalancer.Config{
					AddressPools: []*loadbalancer.AddressPool{
						{
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"84.1.1.1/32",
								"84.1.1.2/32",
							},
						},
					},
				},
				Peers:     []*loadbalancer.Peer{},
				namespace: metallbNamespace,
			},
		},
		{
			name: "two ips acquired, one static ip, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.Pointer("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
				{
					Ipaddress: pointer.Pointer("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
				{
					Ipaddress: pointer.Pointer("84.1.1.3"),
					Name:      "static-ip",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("static"),
				},
			},
			nodes:   []v1.Node{},
			wantErr: nil,
			want: &metalLBConfig{
				Config: loadbalancer.Config{
					AddressPools: []*loadbalancer.AddressPool{
						{
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"84.1.1.1/32",
								"84.1.1.2/32",
							},
						},
						{
							Name:       "internet-static",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"84.1.1.3/32",
							},
						},
					},
				},
				Peers:     []*loadbalancer.Peer{},
				namespace: metallbNamespace,
			},
		},
		{
			name: "connected to internet,storage,dmz and mpls, two ips acquired, one static ip, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.Pointer("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
				{
					Ipaddress: pointer.Pointer("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
				{
					Ipaddress: pointer.Pointer("84.1.1.3"),
					Name:      "static-ip",
					Networkid: pointer.Pointer("internet"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("static"),
				},
				{
					Ipaddress: pointer.Pointer("10.131.44.2"),
					Name:      "static-ip",
					Networkid: pointer.Pointer("shared-storage-network"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("static"),
				},
				{
					Ipaddress: pointer.Pointer("100.127.130.2"),
					Name:      "static-ip",
					Networkid: pointer.Pointer("mpls-network"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("static"),
				},
				{
					Ipaddress: pointer.Pointer("100.127.130.3"),
					Name:      "ephemeral-mpls-ip",
					Networkid: pointer.Pointer("mpls-network"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("ephemeral"),
				},
				{
					Ipaddress: pointer.Pointer("10.129.172.2"),
					Name:      "static-ip",
					Networkid: pointer.Pointer("dmz-network"),
					Projectid: pointer.Pointer("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.Pointer("static"),
				},
			},
			nodes:   []v1.Node{},
			wantErr: nil,
			want: &metalLBConfig{
				Config: loadbalancer.Config{
					AddressPools: []*loadbalancer.AddressPool{
						{
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"84.1.1.1/32",
								"84.1.1.2/32",
							},
						},
						{
							Name:       "internet-static",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"84.1.1.3/32",
							},
						},
						{
							Name:       "shared-storage-network-static",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"10.131.44.2/32",
							},
						},
						{
							Name:       "mpls-network-static",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"100.127.130.2/32",
							},
						},
						{
							Name:       "mpls-network-ephemeral",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"100.127.130.3/32",
							},
						},
						{
							Name:       "dmz-network-static",
							Protocol:   "bgp",
							AutoAssign: pointer.Pointer(false),
							CIDRs: []string{
								"10.129.172.2/32",
							},
						},
					},
				},
				Peers:     []*loadbalancer.Peer{},
				namespace: metallbNamespace,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := &metalLBConfig{}

			err := cfg.PrepareConfig(tt.ips, tt.nws, tt.nodes)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("metalLBConfig.PrepareConfig() error = %v", diff)
				return
			}

			if diff := cmp.Diff(cfg, tt.want, cmpopts.IgnoreUnexported(metalLBConfig{})); diff != "" {
				t.Errorf("metalLBConfig.PrepareConfig() = %v", diff)
			}
		})
	}
}
