package config

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/tag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestMetalLBConfig(t *testing.T) {
	tests := []struct {
		name           string
		nws            sets.Set[string]
		ips            []*models.V1IPResponse
		nodes          []v1.Node
		wantErrmessage *string
		want           *metalLBConfig
	}{
		{
			name: "two ips, one with missing networkID acquired, no nodes",
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
					Name:      "acquired-before",
					Networkid: nil,
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
			},
			nodes:          []v1.Node{},
			wantErrmessage: new("ip has no network id set: 84.1.1.2"),
			want:           nil,
		},
		{
			name: "two ips, one with missing IP acquired, no nodes",
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
					Name:      "acquired-before",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
			},
			nodes:          []v1.Node{},
			wantErrmessage: new("ip address is not set on ip"),
			want:           nil,
		},
		{
			name: "one malformed ip acquired, no nodes",
			nws:  testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: new("84.1.1.1.1"),
					Name:      "acquired-before",
					Networkid: new("internet"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
			},
			nodes:          []v1.Node{},
			wantErrmessage: new("ParseAddr(\"84.1.1.1.1\"): IPv4 address too long"),
			want:           nil,
		},
		{
			name: "one ipv6 acquired, no nodes",
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
			nodes:          []v1.Node{},
			wantErrmessage: nil,
			want: &metalLBConfig{
				Base: &baseConfig{
					AddressPools: addressPools{
						"internet-ephemeral": {
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs:      []string{"2001::a:b:c/128"},
						},
					},
					Peers: nil,
				},
			},
		},
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
			nodes:          []v1.Node{},
			wantErrmessage: nil,
			want: &metalLBConfig{
				Base: &baseConfig{
					AddressPools: addressPools{
						"internet-ephemeral": {
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs:      []string{"84.1.1.1/32"},
						},
					},
					Peers: nil,
				},
			},
		},
		{
			name: "one ip acquired, one node with malformed ASN label",
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
			nodes: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							tag.MachineNetworkPrimaryASN: "abc",
						},
					},
				},
			},
			wantErrmessage: new("unable to parse valid integer from asn annotation: strconv.ParseInt: parsing \"abc\": invalid syntax"),
			want:           nil,
		},
		{
			name: "one ip acquired, one node without ASN label",
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
			nodes: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node-1",
						Labels: nil,
					},
				},
			},
			wantErrmessage: new("node \"node-1\" misses label: machine.metal-stack.io/network.primary.asn"),
			want:           nil,
		},
		{
			name: "one ip acquired, one node with malformed ASN label",
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
			nodes: []v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							tag.MachineNetworkPrimaryASN: "42",
						},
					},
				},
			},
			wantErrmessage: nil,
			want: &metalLBConfig{
				Base: &baseConfig{
					AddressPools: addressPools{
						"internet-ephemeral": {
							Name:       "internet-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs:      []string{"84.1.1.1/32"},
						},
					},
					Peers: nil,
				},
			},
		},

		{
			name: "two ips acquired, no nodes",
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
			},
			nodes:          []v1.Node{},
			wantErrmessage: nil,
			want: &metalLBConfig{
				Base: &baseConfig{
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
					},
					Peers: nil,
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
			nodes:          []v1.Node{},
			wantErrmessage: nil,
			want: &metalLBConfig{
				Base: &baseConfig{
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
					Peers: nil,
				},
			},
		},
		{
			name: "connected to internet,storage,dmz and mpls, two ips acquired, one static ip, no nodes",
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
				{
					Ipaddress: new("10.131.44.2"),
					Name:      "static-ip",
					Networkid: new("shared-storage-network"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("static"),
				},
				{
					Ipaddress: new("100.127.130.2"),
					Name:      "static-ip",
					Networkid: new("mpls-network"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("static"),
				},
				{
					Ipaddress: new("100.127.130.3"),
					Name:      "ephemeral-mpls-ip",
					Networkid: new("mpls-network"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("ephemeral"),
				},
				{
					Ipaddress: new("10.129.172.2"),
					Name:      "static-ip",
					Networkid: new("dmz-network"),
					Projectid: new("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: new("static"),
				},
			},
			nodes:          []v1.Node{},
			wantErrmessage: nil,
			want: &metalLBConfig{
				Base: &baseConfig{
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
						"shared-storage-network-static": {
							Name:       "shared-storage-network-static",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"10.131.44.2/32",
							},
						},
						"mpls-network-static": {
							Name:       "mpls-network-static",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"100.127.130.2/32",
							},
						},
						"mpls-network-ephemeral": {
							Name:       "mpls-network-ephemeral",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"100.127.130.3/32",
							},
						},
						"dmz-network-static": {
							Name:       "dmz-network-static",
							Protocol:   "bgp",
							AutoAssign: new(false),
							CIDRs: []string{
								"10.129.172.2/32",
							},
						},
					},
					Peers: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(LoadBalancerTypeMetalLB, tt.ips, tt.nws, tt.nodes, nil, nil)
			if err != nil {
				if diff := cmp.Diff(err.Error(), *tt.wantErrmessage); diff != "" {
					t.Errorf("error = %v", diff)
				}
				return
			}

			if diff := cmp.Diff(cfg, tt.want, cmpopts.IgnoreUnexported(metalLBConfig{})); diff != "" {
				t.Errorf("diff = %v", diff)
			}
		})
	}
}
