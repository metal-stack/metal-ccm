package loadbalancer

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
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
		want    map[string]interface{}
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
			want: map[string]interface{}{
				"address-pools": []map[string]interface{}{
					{
						"addresses": []string{
							"84.1.1.1/32",
						},
						"auto-assign": false,
						"name":        "internet-ephemeral",
						"protocol":    "bgp",
					},
				},
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
			want: map[string]interface{}{
				"address-pools": []map[string]interface{}{
					{
						"addresses": []string{
							"84.1.1.1/32",
							"84.1.1.2/32",
						},
						"auto-assign": false,
						"name":        "internet-ephemeral",
						"protocol":    "bgp",
					},
				},
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
			want: map[string]interface{}{
				"address-pools": []map[string]interface{}{
					{
						"addresses": []string{
							"84.1.1.1/32",
							"84.1.1.2/32",
						},
						"auto-assign": false,
						"name":        "internet-ephemeral",
						"protocol":    "bgp",
					},
					{
						"addresses": []string{
							"84.1.1.3/32",
						},
						"auto-assign": false,
						"name":        "internet-static",
						"protocol":    "bgp",
					},
				},
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
			want: map[string]interface{}{
				"address-pools": []map[string]interface{}{
					{
						"addresses": []string{
							"84.1.1.1/32",
							"84.1.1.2/32",
						},
						"auto-assign": false,
						"name":        "internet-ephemeral",
						"protocol":    "bgp",
					},
					{
						"addresses": []string{
							"84.1.1.3/32",
						},
						"auto-assign": false,
						"name":        "internet-static",
						"protocol":    "bgp",
					},
					{
						"addresses": []string{
							"10.131.44.2/32",
						},
						"auto-assign": false,
						"name":        "shared-storage-network-static",
						"protocol":    "bgp",
					},
					{
						"addresses": []string{
							"100.127.130.2/32",
						},
						"auto-assign": false,
						"name":        "mpls-network-static",
						"protocol":    "bgp",
					},
					{
						"addresses": []string{
							"100.127.130.3/32",
						},
						"auto-assign": false,
						"name":        "mpls-network-ephemeral",
						"protocol":    "bgp",
					},
					{
						"addresses": []string{
							"10.129.172.2/32",
						},
						"auto-assign": false,
						"name":        "dmz-network-static",
						"protocol":    "bgp",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := &MetalLBConfig{}

			err := cfg.CalculateConfig(tt.ips, tt.nws, tt.nodes)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("MetalLBConfig.CalculateConfig() error = %v", diff)
				return
			}

			yaml, err := cfg.ToYAML()
			require.NoError(t, err)

			if diff := cmp.Diff(yaml, mustYAML(tt.want)); diff != "" {
				t.Errorf("MetalLBConfig.CalculateConfig() = %v", diff)
			}
		})
	}
}

func mustYAML(data interface{}) string {
	res, _ := yaml.Marshal(data)
	return string(res)
}
