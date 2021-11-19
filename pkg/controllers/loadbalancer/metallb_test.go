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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

var (
	testNetworks = map[string]*models.V1NetworkResponse{
		"internet": {
			Destinationprefixes: []string{"0.0.0.0/0"},
			ID:                  pointer.StringPtr("internet"),
			Labels: map[string]string{
				"network.metal-stack.io/default":          "",
				"network.metal-stack.io/default-external": "",
			},
			Nat:             pointer.BoolPtr(true),
			Parentnetworkid: "",
			Partitionid:     "",
			Privatesuper:    pointer.BoolPtr(false),
			Projectid:       "",
			Shared:          false,
			Underlay:        pointer.BoolPtr(false),
			Vrf:             104009,
			Vrfshared:       false,
		},
		"tenant-super-network-partition-a": {
			Destinationprefixes: []string{},
			ID:                  pointer.StringPtr("tenant-super-network-partition-a"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(false),
			Parentnetworkid:     "",
			Partitionid:         "",
			Privatesuper:        pointer.BoolPtr(true),
			Projectid:           "",
			Shared:              false,
			Underlay:            pointer.BoolPtr(false),
			Vrf:                 0,
			Vrfshared:           false,
		},
		"underlay-partition-a": {
			Destinationprefixes: []string{},
			ID:                  pointer.StringPtr("underlay-partition-a"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(false),
			Parentnetworkid:     "",
			Partitionid:         "",
			Privatesuper:        pointer.BoolPtr(false),
			Projectid:           "",
			Shared:              false,
			Underlay:            pointer.BoolPtr(true),
			Vrf:                 0,
			Vrfshared:           false,
		},
		"this-cluster-private-network": {
			Destinationprefixes: []string{"10.129.28.0/22"},
			ID:                  pointer.StringPtr("this-cluster-private-network"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(false),
			Parentnetworkid:     "tenant-super-network-partition-a",
			Partitionid:         "partition-a",
			Privatesuper:        pointer.BoolPtr(false),
			Projectid:           "project-a",
			Shared:              false,
			Underlay:            pointer.BoolPtr(false),
			Vrf:                 30,
			Vrfshared:           false,
		},
		"foreign-cluster-private-network": {
			Destinationprefixes: []string{"10.128.244.0/22"},
			ID:                  pointer.StringPtr("foreign-cluster-private-network"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(false),
			Parentnetworkid:     "tenant-super-network-partition-a",
			Partitionid:         "partition-a",
			Privatesuper:        pointer.BoolPtr(false),
			Projectid:           "project-b",
			Shared:              false,
			Underlay:            pointer.BoolPtr(false),
			Vrf:                 40,
			Vrfshared:           false,
		},

		"shared-storage-network": {
			Destinationprefixes: []string{""},
			Prefixes:            []string{"10.131.44.0/22"},
			ID:                  pointer.StringPtr("shared-storage-network"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(false),
			Parentnetworkid:     "tenant-super-network-partition-a",
			Partitionid:         "partition-a",
			Privatesuper:        pointer.BoolPtr(false),
			Projectid:           "project-c",
			Shared:              true,
			Underlay:            pointer.BoolPtr(false),
			Vrf:                 20,
			Vrfshared:           false,
		},
		"mpls-network": {
			Destinationprefixes: []string{"100.127.0.0/16"},
			Prefixes:            []string{"100.127.130.0/23"},
			ID:                  pointer.StringPtr("mpls-network"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(true),
			Parentnetworkid:     "",
			Partitionid:         "",
			Privatesuper:        pointer.BoolPtr(false),
			Projectid:           "",
			Shared:              false,
			Underlay:            pointer.BoolPtr(false),
			Vrf:                 104010,
			Vrfshared:           false,
		},
		"dmz-network": {
			Destinationprefixes: []string{"0.0.0.0/0"},
			Prefixes:            []string{"10.129.172.0/22"},
			ID:                  pointer.StringPtr("dmz-network"),
			Labels:              map[string]string{},
			Nat:                 pointer.BoolPtr(false),
			Parentnetworkid:     "tenant-super-network-partition-a",
			Partitionid:         "partition-a",
			Privatesuper:        pointer.BoolPtr(false),
			Projectid:           "project-a",
			Shared:              true,
			Underlay:            pointer.BoolPtr(false),
			Vrf:                 52,
			Vrfshared:           false,
		},
	}
)

func TestMetalLBConfig_CalculateConfig(t *testing.T) {
	tests := []struct {
		name             string
		defaultNetworkID string
		nws              map[string]*models.V1NetworkResponse
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
