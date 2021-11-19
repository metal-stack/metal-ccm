package loadbalancer

import (
	"fmt"
	"log"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

var (
	testNetworks = sets.NewString(
		"internet",
		"foreign-cluster-private-network",
		"shared-storage-network",
		"mpls-network",
		"dmz-network",
	)
)

func TestMetalLBConfig_CalculateConfig(t *testing.T) {
	tests := []struct {
		name             string
		defaultNetworkID string
		nws              sets.String
		ips              []*models.V1IPResponse
		nodes            []v1.Node
		wantErr          error
		want             map[string]interface{}
	}{
		{
			name:             "one ip acquired, no nodes",
			defaultNetworkID: "internet",
			nws:              testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.StringPtr("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
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
						"auto-assign": true,
						"name":        "internet-ephemeral",
						"protocol":    "bgp",
					},
				},
			},
		},
		{
			name:             "two ips acquired, no nodes",
			defaultNetworkID: "internet",
			nws:              testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.StringPtr("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
				},
				{
					Ipaddress: pointer.StringPtr("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
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
						"auto-assign": true,
						"name":        "internet-ephemeral",
						"protocol":    "bgp",
					},
				},
			},
		},
		{
			name:             "two ips acquired, one static ip, no nodes",
			defaultNetworkID: "internet",
			nws:              testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.StringPtr("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
				},
				{
					Ipaddress: pointer.StringPtr("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
				},
				{
					Ipaddress: pointer.StringPtr("84.1.1.3"),
					Name:      "static-ip",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("static"),
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
						"auto-assign": true,
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
			name:             "connected to internet,storage,dmz and mpls, two ips acquired, one static ip, no nodes",
			defaultNetworkID: "internet",
			nws:              testNetworks,
			ips: []*models.V1IPResponse{
				{
					Ipaddress: pointer.StringPtr("84.1.1.1"),
					Name:      "acquired-before",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
				},
				{
					Ipaddress: pointer.StringPtr("84.1.1.2"),
					Name:      "acquired-before-2",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
				},
				{
					Ipaddress: pointer.StringPtr("84.1.1.3"),
					Name:      "static-ip",
					Networkid: pointer.StringPtr("internet"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("static"),
				},
				{
					Ipaddress: pointer.StringPtr("10.131.44.2"),
					Name:      "static-ip",
					Networkid: pointer.StringPtr("shared-storage-network"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("static"),
				},
				{
					Ipaddress: pointer.StringPtr("100.127.130.2"),
					Name:      "static-ip",
					Networkid: pointer.StringPtr("mpls-network"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("static"),
				},
				{
					Ipaddress: pointer.StringPtr("100.127.130.3"),
					Name:      "ephemeral-mpls-ip",
					Networkid: pointer.StringPtr("mpls-network"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("ephemeral"),
				},
				{
					Ipaddress: pointer.StringPtr("10.129.172.2"),
					Name:      "static-ip",
					Networkid: pointer.StringPtr("dmz-network"),
					Projectid: pointer.StringPtr("project-a"),
					Tags: []string{
						fmt.Sprintf("%s=%s", tag.ClusterID, "this-cluster"),
					},
					Type: pointer.StringPtr("static"),
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
						"auto-assign": true,
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
			cfg := &MetalLBConfig{
				logger:           log.New(log.Writer(), "testing", log.LstdFlags),
				defaultNetworkID: tt.defaultNetworkID,
			}

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
