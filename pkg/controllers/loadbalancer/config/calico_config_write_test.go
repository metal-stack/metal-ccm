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

	// the bgp filter restricts the peering with the frr daemon on the node to the load balancer ips.
	bgpFilter := &calicov3.BGPFilter{}
	requireGet(t, ctx, c, bgpFilterName, bgpFilter)

	wantFilterSpec := calicov3.BGPFilterSpec{
		// no route announced by frr may be imported, otherwise calico programs the routing table
		// of the metal-stack networking infrastructure into the kernel of the node.
		ImportV4: []calicov3.BGPFilterRuleV4{
			{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
		},
		ExportV4: []calicov3.BGPFilterRuleV4{
			{CIDR: "84.1.1.1/32", MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Accept},
			{CIDR: "84.1.1.2/32", MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Accept},
			{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
		},
	}
	if diff := cmp.Diff(wantFilterSpec, bgpFilter.Spec, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("bgp filter spec diff = %s", diff)
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
			Filters:       []string{bgpFilterName},
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

	// even without any load balancer ip the filter is created, but it must not accept a single
	// route in either direction.
	bgpFilter := &calicov3.BGPFilter{}
	requireGet(t, ctx, c, bgpFilterName, bgpFilter)

	wantFilterSpec := calicov3.BGPFilterSpec{
		ImportV4: []calicov3.BGPFilterRuleV4{
			{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
		},
		ExportV4: []calicov3.BGPFilterRuleV4{
			{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
		},
	}
	if diff := cmp.Diff(wantFilterSpec, bgpFilter.Spec, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("bgp filter spec diff = %s", diff)
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

func TestCalicoConfigWriteBGPFilter(t *testing.T) {
	ctx := context.Background()

	base := &baseConfig{
		AddressPools: addressPools{
			"internet-ephemeral": {
				Name:       "internet-ephemeral",
				Protocol:   "bgp",
				AutoAssign: new(false),
				CIDRs:      []string{"84.1.1.2/32", "2001::a:b:c/128"},
			},
			"internet-static": {
				Name:       "internet-static",
				Protocol:   "bgp",
				AutoAssign: new(false),
				CIDRs:      []string{"84.1.1.1/32", "2001::2/128"},
			},
		},
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(AddCalicoToScheme(scheme))

	c := crfake.NewClientBuilder().WithScheme(scheme).Build()

	cfg := newCalicoConfig(base, c)

	// writing the resources twice ensures they are updated in place instead of being created twice
	for range 2 {
		err := cfg.WriteCRs(ctx)
		if err != nil {
			t.Fatalf("WriteCRs: %v", err)
		}
	}

	bgpFilter := &calicov3.BGPFilter{}
	requireGet(t, ctx, c, bgpFilterName, bgpFilter)

	// ipv6 load balancer ips are not announced through the ipv4 peerings with the frr daemon on the
	// node, therefore they must not show up in the filter. The cidrs have to be sorted because they
	// are collected from a map.
	wantSpec := calicov3.BGPFilterSpec{
		ImportV4: []calicov3.BGPFilterRuleV4{
			{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
		},
		ExportV4: []calicov3.BGPFilterRuleV4{
			{CIDR: "84.1.1.1/32", MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Accept},
			{CIDR: "84.1.1.2/32", MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Accept},
			{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
		},
	}
	if diff := cmp.Diff(wantSpec, bgpFilter.Spec); diff != "" {
		t.Errorf("bgp filter spec diff = %s", diff)
	}

	// load balancer ips which are not acquired anymore must not be announced any longer
	ephemeral := base.AddressPools["internet-ephemeral"]
	ephemeral.CIDRs = []string{"84.1.1.3/32"}
	base.AddressPools["internet-ephemeral"] = ephemeral
	delete(base.AddressPools, "internet-static")

	err := cfg.WriteCRs(ctx)
	if err != nil {
		t.Fatalf("WriteCRs: %v", err)
	}

	bgpFilter = &calicov3.BGPFilter{}
	requireGet(t, ctx, c, bgpFilterName, bgpFilter)

	wantSpec.ExportV4 = []calicov3.BGPFilterRuleV4{
		{CIDR: "84.1.1.3/32", MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Accept},
		{CIDR: allIPv4, MatchOperator: calicov3.MatchOperatorIn, Action: calicov3.Reject},
	}
	if diff := cmp.Diff(wantSpec, bgpFilter.Spec); diff != "" {
		t.Errorf("bgp filter spec diff after update = %s", diff)
	}
}
