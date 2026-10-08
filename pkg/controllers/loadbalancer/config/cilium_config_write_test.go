package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metal-stack/metal-lib/pkg/tag"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNodeA = "node-a"
	testNodeB = "node-b"
)

func testPeer(asn uint32, nodeName, address string) *peer {
	return &peer{
		MyASN:    asn,
		ASN:      asn,
		Address:  address,
		NodeName: nodeName,
		NodeSelector: metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "kubernetes.io/hostname",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{nodeName},
				},
			},
		},
	}
}

func TestCiliumConfigWriteCRs(t *testing.T) {
	ctx := context.Background()

	base := &baseConfig{
		Peers: []*peer{
			testPeer(70000, testNodeA, "10.0.0.1"),
			testPeer(70001, testNodeB, "10.0.0.2"),
		},
		AddressPools: addressPools{
			"internet-ephemeral": {
				Name:       "internet-ephemeral",
				Protocol:   "bgp",
				AutoAssign: new(false),
				CIDRs:      []string{"84.1.1.1/32", "84.1.1.2/32"},
			},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(ciliumv2.AddToScheme(scheme))

	staleClusterConfig := &ciliumv2.CiliumBGPClusterConfig{ObjectMeta: metav1.ObjectMeta{Name: "99999"}}
	staleOverride := &ciliumv2.CiliumBGPNodeConfigOverride{ObjectMeta: metav1.ObjectMeta{Name: "stale-node"}}
	stalePool := &ciliumv2.CiliumLoadBalancerIPPool{ObjectMeta: metav1.ObjectMeta{Name: "stale-pool"}}

	c := crfake.NewClientBuilder().WithScheme(scheme).WithObjects(staleClusterConfig, staleOverride, stalePool).Build()

	cfg := newCiliumConfig(base, c)

	// writing the resources twice ensures they are updated in place instead of being created twice
	for range 2 {
		err := cfg.WriteCRs(ctx)
		if err != nil {
			t.Fatalf("WriteCRs: %v", err)
		}
	}

	// the cluster wide peer config
	peerConfig := &ciliumv2.CiliumBGPPeerConfig{}
	requireGet(t, ctx, c, peerConfigName, peerConfig)

	// the cluster wide peer config selects the cluster wide advertisements with this label
	wantAdvertisementLabels := map[string]string{"advertise": "bgp"}

	wantPeerConfig := ciliumv2.CiliumBGPPeerConfigSpec{
		GracefulRestart: &ciliumv2.CiliumBGPNeighborGracefulRestart{Enabled: true},
		Families: []ciliumv2.CiliumBGPFamilyWithAdverts{
			{
				CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
				Advertisements:  &slimv1.LabelSelector{MatchLabels: wantAdvertisementLabels},
			},
			{
				CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv6", Safi: "unicast"},
				Advertisements:  &slimv1.LabelSelector{MatchLabels: wantAdvertisementLabels},
			},
		},
	}
	if diff := cmp.Diff(wantPeerConfig, peerConfig.Spec, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("peer config spec diff = %s", diff)
	}

	// the cluster wide advertisements
	advertisements := &ciliumv2.CiliumBGPAdvertisement{}
	requireGet(t, ctx, c, advertisementName, advertisements)

	if diff := cmp.Diff(wantAdvertisementLabels, advertisements.Labels); diff != "" {
		t.Errorf("advertisement labels diff = %s", diff)
	}

	wantAdvertisementSpec := ciliumv2.CiliumBGPAdvertisementSpec{
		Advertisements: []ciliumv2.BGPAdvertisement{
			{
				AdvertisementType: ciliumv2.BGPPodCIDRAdvert,
			},
			{
				AdvertisementType: ciliumv2.BGPServiceAdvert,
				Service: &ciliumv2.BGPServiceOptions{
					Addresses: []ciliumv2.BGPServiceAddressType{ciliumv2.BGPLoadBalancerIPAddr},
				},
				Selector: &slimv1.LabelSelector{
					MatchExpressions: []slimv1.LabelSelectorRequirement{
						{
							Key:      ciliumv2.BGPLoadBalancerClass,
							Operator: slimv1.LabelSelectorOpNotIn,
							Values:   []string{"ignore"},
						},
					},
				},
			},
		},
	}
	if diff := cmp.Diff(wantAdvertisementSpec, advertisements.Spec, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("advertisement spec diff = %s", diff)
	}

	// one cluster config and one node config override per peer
	for _, p := range base.Peers {
		clusterConfig := &ciliumv2.CiliumBGPClusterConfig{}
		requireGet(t, ctx, c, fmt.Sprintf("%d", p.ASN), clusterConfig)

		wantClusterConfig := ciliumv2.CiliumBGPClusterConfigSpec{
			NodeSelector: &slimv1.LabelSelector{
				MatchExpressions: []slimv1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/hostname",
						Operator: slimv1.LabelSelectorOpIn,
						Values:   []string{p.NodeName},
					},
				},
			},
			BGPInstances: []ciliumv2.CiliumBGPInstance{
				{
					Name:     fmt.Sprintf("instance-%d", p.ASN),
					LocalASN: new(int64(p.MyASN)),
					Peers: []ciliumv2.CiliumBGPPeer{
						{
							Name:        fmt.Sprintf("peer-%d", p.ASN),
							PeerAddress: new("127.0.0.1"),
							PeerASN:     new(int64(p.ASN)),
							PeerConfigRef: &ciliumv2.PeerConfigReference{
								Name: peerConfigName,
							},
						},
					},
				},
			},
		}
		if diff := cmp.Diff(wantClusterConfig, clusterConfig.Spec, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("cluster config %q spec diff = %s", clusterConfig.Name, diff)
		}

		override := &ciliumv2.CiliumBGPNodeConfigOverride{}
		requireGet(t, ctx, c, p.NodeName, override)

		wantOverride := ciliumv2.CiliumBGPNodeConfigOverrideSpec{
			BGPInstances: []ciliumv2.CiliumBGPNodeConfigInstanceOverride{
				{
					Name:     fmt.Sprintf("instance-%d", p.ASN),
					RouterID: new("127.0.0.1"),
				},
			},
		}
		if diff := cmp.Diff(wantOverride, override.Spec, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("node config override %q spec diff = %s", override.Name, diff)
		}
	}

	// one ip pool per address pool
	for _, pool := range base.AddressPools {
		ipPool := &ciliumv2.CiliumLoadBalancerIPPool{}
		requireGet(t, ctx, c, pool.Name, ipPool)

		var wantBlocks []ciliumv2.CiliumLoadBalancerIPPoolIPBlock
		for _, cidr := range pool.CIDRs {
			wantBlocks = append(wantBlocks, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{
				Cidr: ciliumv2.IPv4orIPv6CIDR(cidr),
			})
		}
		if diff := cmp.Diff(wantBlocks, ipPool.Spec.Blocks, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("ip pool %q spec diff = %s", ipPool.Name, diff)
		}
	}

	// resources which are not managed anymore are removed
	requireNotFound(t, ctx, c, staleClusterConfig)
	requireNotFound(t, ctx, c, staleOverride)
	requireNotFound(t, ctx, c, stalePool)
}

func TestCiliumConfigPeersFromNodes(t *testing.T) {
	nodes := []v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   testNodeA,
				Labels: map[string]string{tag.MachineNetworkPrimaryASN: "70000"},
			},
			Status: v1.NodeStatus{
				Addresses: []v1.NodeAddress{
					{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		},
	}

	bc, err := newBaseConfig(nil, sets.New[string](), nodes)
	if err != nil {
		t.Fatalf("newBaseConfig: %v", err)
	}

	want := []*peer{testPeer(70000, testNodeA, "10.0.0.1")}
	if diff := cmp.Diff(want, bc.Peers, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("peers diff = %s", diff)
	}
}

func requireGet(t *testing.T, ctx context.Context, c client.Client, name string, obj client.Object) {
	t.Helper()

	err := c.Get(ctx, client.ObjectKey{Name: name}, obj)
	if err != nil {
		t.Fatalf("failed to get %T %q: %v", obj, name, err)
	}
}

func requireNotFound(t *testing.T, ctx context.Context, c client.Client, obj client.Object) {
	t.Helper()

	err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected %T %q to be removed, got: %v", obj, obj.GetName(), err)
	}
}
