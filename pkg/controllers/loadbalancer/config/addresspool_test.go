package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

func Test_addressPool_appendIP(t *testing.T) {
	tests := []struct {
		name     string
		existing addressPool
		ip       *apiv2.IP
		want     addressPool
		wantErr  error
	}{
		{
			name: "append ipv4 to empty pool",
			existing: addressPool{
				CIDRs: nil,
			},
			ip: &apiv2.IP{
				Ip: "192.168.2.1",
			},
			want: addressPool{
				CIDRs: []string{"192.168.2.1/32"},
			},
		},
		{
			name: "don't append if already contained",
			existing: addressPool{
				CIDRs: []string{"192.168.2.1/32"},
			},
			ip: &apiv2.IP{
				Ip: "192.168.2.1",
			},
			want: addressPool{
				CIDRs: []string{"192.168.2.1/32"},
			},
		},
		{
			name: "append ipv6 to pool",
			existing: addressPool{
				CIDRs: []string{"192.168.2.1/32"},
			},
			ip: &apiv2.IP{
				Ip: "2001::7",
			},
			want: addressPool{
				CIDRs: []string{"192.168.2.1/32", "2001::7/128"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.existing.appendIP(tt.ip)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("error = %v", diff)
				return
			}

			if diff := cmp.Diff(tt.existing, tt.want); diff != "" {
				t.Errorf("diff = %v", diff)
			}
		})
	}
}

func Test_addressPools_addPoolIP(t *testing.T) {
	tests := []struct {
		name     string
		poolName string
		ip       *apiv2.IP
		existing addressPools
		want     addressPools
		wantErr  error
	}{
		{
			name:     "append new pool",
			poolName: "my-pool-static",
			ip: &apiv2.IP{
				Ip:   "2001::7",
				Type: apiv2.IPType_IP_TYPE_STATIC,
			},
			existing: addressPools{},
			want: addressPools{
				"my-pool-static": addressPool{
					Name:       "my-pool-static",
					Protocol:   bgpProtocol,
					AutoAssign: pointer.Pointer(false),
					CIDRs:      []string{"2001::7/128"},
				},
			},
		},
		{
			name:     "append to existing pool",
			poolName: "my-pool-static",
			ip: &apiv2.IP{
				Ip:   "2001::8",
				Type: apiv2.IPType_IP_TYPE_STATIC,
			},
			existing: addressPools{
				"my-pool-ephemeral": addressPool{
					Name:       "my-pool-ephemeral",
					Protocol:   bgpProtocol,
					AutoAssign: pointer.Pointer(false),
					CIDRs:      []string{"192.168.2.1/32"},
				},
				"my-pool-static": addressPool{
					Name:       "my-pool-static",
					Protocol:   bgpProtocol,
					AutoAssign: pointer.Pointer(false),
					CIDRs:      []string{"2001::7/128"},
				},
			},
			want: addressPools{
				"my-pool-ephemeral": addressPool{
					Name:       "my-pool-ephemeral",
					Protocol:   bgpProtocol,
					AutoAssign: pointer.Pointer(false),
					CIDRs:      []string{"192.168.2.1/32"},
				},
				"my-pool-static": addressPool{
					Name:       "my-pool-static",
					Protocol:   bgpProtocol,
					AutoAssign: pointer.Pointer(false),
					CIDRs:      []string{"2001::7/128", "2001::8/128"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.existing.addPoolIP(tt.poolName, tt.ip)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("error = %v", diff)
				return
			}

			if diff := cmp.Diff(tt.existing, tt.want); diff != "" {
				t.Errorf("diff = %v", diff)
			}
		})
	}
}
