package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/projectcalico/api/pkg/lib/numorstring"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCalicoConfigWriteCRs(t *testing.T) {
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
				CIDRs:      []string{"84.1.1.2/32", "84.1.1.1/32"},
			},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(AddCalicoToScheme(scheme))

	// a peering which was managed by metal-ccm before, but is not contained in the
	// desired configuration anymore.
	stalePeer := &calicov3.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "peer-12345",
			Labels: map[string]string{managedByLabelKey: managedByLabelValue},
		},
	}
	// a peering which is not managed by metal-ccm, this one must not be touched.
	foreignPeer := &calicov3.BGPPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "peer-4711",
		},
	}

	c := crfake.NewClientBuilder().WithScheme(scheme).WithObjects(stalePeer, foreignPeer).Build()

	cfg := newCalicoConfig(base, c)

	// writing the resources twice ensures they are updated in place instead of being created twice
	for range 2 {
		err := cfg.WriteCRs(ctx)
		if err != nil {
			t.Fatalf("WriteCRs: %v", err)
		}
	}

	// the load balancer ips are configured in the default bgp configuration,
	// the cidrs have to be sorted because they are collected from a map.
	bgpConfiguration := &calicov3.BGPConfiguration{}
	requireGet(t, ctx, c, bgpConfigurationName, bgpConfiguration)

	wantBlocks := []calicov3.ServiceLoadBalancerIPBlock{
		{CIDR: "84.1.1.1/32"},
		{CIDR: "84.1.1.2/32"},
	}
	if diff := cmp.Diff(wantBlocks, bgpConfiguration.Spec.ServiceLoadBalancerIPs, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("bgp configuration spec diff = %s", diff)
	}

	// one peer per node which peers with the bgp daemon running on the node itself
	for _, p := range base.Peers {
		bgpPeer := &calicov3.BGPPeer{}
		requireGet(t, ctx, c, bgpPeerName(p), bgpPeer)

		wantSpec := calicov3.BGPPeerSpec{
			Node:          p.NodeName,
			PeerIP:        frrPeerAddress,
			ASNumber:      numorstring.ASNumber(p.ASN),
			LocalASNumber: new(numorstring.ASNumber(p.MyASN)),
			SourceAddress: calicov3.SourceAddressNone,
		}
		if diff := cmp.Diff(wantSpec, bgpPeer.Spec, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("bgp peer %q spec diff = %s", bgpPeer.Name, diff)
		}

		wantLabels := map[string]string{managedByLabelKey: managedByLabelValue}
		if diff := cmp.Diff(wantLabels, bgpPeer.Labels, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("bgp peer %q labels diff = %s", bgpPeer.Name, diff)
		}
	}

	// peers which are not managed anymore are removed
	requireNotFound(t, ctx, c, stalePeer)

	// peers which are managed by someone else are kept
	requireGet(t, ctx, c, foreignPeer.Name, &calicov3.BGPPeer{})

	// a node which is gone does not keep its peering
	base.Peers = base.Peers[:1]

	err := cfg.WriteCRs(ctx)
	if err != nil {
		t.Fatalf("WriteCRs: %v", err)
	}

	requireNotFound(t, ctx, c, &calicov3.BGPPeer{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("peer-%d", 70001)}})
	requireGet(t, ctx, c, foreignPeer.Name, &calicov3.BGPPeer{})
}

func TestCalicoConfigWriteCRsEmpty(t *testing.T) {
	ctx := context.Background()

	base := &baseConfig{
		AddressPools: addressPools{},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(AddCalicoToScheme(scheme))

	c := crfake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := newCalicoConfig(base, c)

	err := cfg.WriteCRs(ctx)
	if err != nil {
		t.Fatalf("WriteCRs: %v", err)
	}

	// even without any address pool and peer the default bgp configuration is created,
	// this makes sure a leftover service load balancer ip block from a previous
	// metal-ccm run is removed again.
	bgpConfiguration := &calicov3.BGPConfiguration{}
	requireGet(t, ctx, c, bgpConfigurationName, bgpConfiguration)

	if diff := cmp.Diff([]calicov3.ServiceLoadBalancerIPBlock(nil), bgpConfiguration.Spec.ServiceLoadBalancerIPs, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("bgp configuration spec diff = %s", diff)
	}

	peers := &calicov3.BGPPeerList{}
	err = c.List(ctx, peers)
	if err != nil {
		t.Fatalf("list bgppeers: %v", err)
	}

	if len(peers.Items) != 0 {
		t.Errorf("expected no bgppeers, got %d", len(peers.Items))
	}
}
