package config

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/projectcalico/api/pkg/lib/numorstring"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	LoadBalancerTypeCalico LoadBalancerType = "calico"

	// calicoAPIGroup and calicoAPIVersion are the api group and version of the calico custom
	// resources which are served by the kubernetes api server. The types of the
	// github.com/projectcalico/api module are declared for the projectcalico.org/v3 group, which
	// is only served if the calico api server is deployed. A default calico installation only
	// provides the custom resources, therefore the calico types have to be registered with this
	// group version.
	calicoAPIGroup   = "crd.projectcalico.org"
	calicoAPIVersion = "v1"

	// bgpConfigurationName is the name of the default BGPConfiguration of a calico installation.
	// Calico has only this one cluster wide default BGPConfiguration, so the load balancer ips have
	// to be configured in this resource.
	bgpConfigurationName = "default"

	// frrPeerAddress is the address of the bgp daemon (frr) running on the node itself. Calico peers
	// with this address to announce the load balancer ips to the metal-stack networking infrastructure.
	frrPeerAddress = "127.0.0.1"

	// bgpFilterName is the name of the BGPFilter which is referenced by all BGPPeers maintained by
	// metal-ccm. It restricts the peering with the frr daemon on the node to the load balancer ips.
	// It must exist before the first BGPPeer is written, otherwise calico peers without any filter.
	bgpFilterName = "metal-ccm-bgp-filter"

	// allIPv4 is used to match all ipv4 routes in a BGPFilter rule, because every ipv4 prefix is
	// contained in 0.0.0.0/0.
	allIPv4 = "0.0.0.0/0"

	// managedByLabelKey and managedByLabelValue mark the BGPPeer resources which are maintained by
	// metal-ccm. Peerings which are not labeled this way are managed by someone else and are left
	// untouched by metal-ccm.
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "metal-ccm"
)

// calicoGroupVersion is the group version of the calico custom resources, see calicoAPIGroup.
var calicoGroupVersion = schema.GroupVersion{Group: calicoAPIGroup, Version: calicoAPIVersion}

// AddCalicoToScheme registers the calico resources which are maintained by metal-ccm with the group
// version the calico custom resources are served in, see calicoAPIGroup.
func AddCalicoToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(calicoGroupVersion,
		&calicov3.BGPPeer{},
		&calicov3.BGPPeerList{},
		&calicov3.BGPConfiguration{},
		&calicov3.BGPConfigurationList{},
		&calicov3.BGPFilter{},
		&calicov3.BGPFilterList{},
	)
	metav1.AddToGroupVersion(scheme, calicoGroupVersion)

	return nil
}

type calicoConfig struct {
	base   *baseConfig
	client client.Client
}

func newCalicoConfig(base *baseConfig, c client.Client) *calicoConfig {
	return &calicoConfig{base: base, client: c}
}

func (c *calicoConfig) WriteCRs(ctx context.Context) error {
	// the bgp filter is written first, because the bgp peers refer to it. A peering without filter
	// would import all routes announced by the frr daemon on the node, see writeBGPFilter.
	err := c.writeBGPFilter(ctx)
	if err != nil {
		return fmt.Errorf("failed to write bgpfilter resource %w", err)
	}

	err = c.writeBGPConfiguration(ctx)
	if err != nil {
		return fmt.Errorf("failed to write bgpconfiguration resource %w", err)
	}

	err = c.writeBGPPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to write bgppeer resources %w", err)
	}

	return nil
}

// writeBGPFilter writes the BGPFilter which is referenced by all BGPPeers maintained by metal-ccm.
//
// Peering calico with the frr daemon running on the node itself gives the cluster access to the
// routing table of the metal-stack networking infrastructure, therefore the peering has to be
// restricted in both directions:
//
// Import: frr announces all routes it knows about to calico, including the default route learned
// from the leaf. Calico evaluates the import filter of node peers as "accept everything", so bird
// imports these routes and programs them into the kernel routing table of the node. This breaks the
// routing of the node itself and frr redistributes the routes back into the metal-stack networking
// infrastructure, where they can poison the routing tables of other machines. Consequently no route
// may be imported at all.
//
// Export: calico announces the load balancer ips, which are the only addresses of the cluster that
// are supposed to be routed by the metal-stack networking infrastructure. The bgp filter is
// evaluated before calico's own export logic, so accepting the load balancer ips and rejecting
// everything else makes sure no address pool or workload route is announced to frr.
func (c *calicoConfig) writeBGPFilter(ctx context.Context) error {
	bgpFilter := &calicov3.BGPFilter{
		TypeMeta: metav1.TypeMeta{
			APIVersion: calicoGroupVersion.String(),
			Kind:       calicov3.KindBGPFilter,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bgpFilterName,
		},
	}

	res, err := controllerutil.CreateOrUpdate(ctx, c.client, bgpFilter, func() error {
		bgpFilter.Spec = calicov3.BGPFilterSpec{
			ImportV4: rejectAllRules(),
			// the accept rules have to be evaluated before the reject rule, otherwise no load
			// balancer ip is announced at all.
			ExportV4: append(acceptLoadBalancerIPRules(c.base.AddressPools), rejectAllRules()...),
		}
		return nil
	})
	if err != nil {
		return err
	}

	if res != controllerutil.OperationResultNone {
		klog.Infof("bgpfilter %q: %v", bgpFilter.Name, res)
	}

	return nil
}

// acceptLoadBalancerIPRules returns one accept rule per load balancer ip which is currently acquired
// by the cluster. Only ipv4 addresses are considered, because the peers written by metal-ccm are
// ipv4 peerings, there is no peering with an ipv6 address of the frr daemon.
func acceptLoadBalancerIPRules(pools addressPools) []calicov3.BGPFilterRuleV4 {
	// the address pools are stored in a map, so the cidrs have to be sorted to get a deterministic result.
	var cidrs []string
	for _, pool := range pools {
		for _, cidr := range pool.CIDRs {
			prefix, err := netip.ParsePrefix(cidr)
			if err != nil {
				klog.Warningf("skipping cidr %q: %v", cidr, err)
				continue
			}

			if !prefix.Addr().Is4() {
				continue
			}

			cidrs = append(cidrs, prefix.String())
		}
	}
	slices.Sort(cidrs)
	cidrs = slices.Compact(cidrs)

	rules := make([]calicov3.BGPFilterRuleV4, 0, len(cidrs))
	for _, cidr := range cidrs {
		rules = append(rules, calicov3.BGPFilterRuleV4{
			CIDR: cidr,
			// the match operator "In" matches all routes which are contained in the cidr,
			// therefore a load balancer ip is only matched by its host route.
			MatchOperator: calicov3.MatchOperatorIn,
			Action:        calicov3.Accept,
		})
	}

	return rules
}

// rejectAllRules returns a single rule which rejects every ipv4 route, including the default route.
func rejectAllRules() []calicov3.BGPFilterRuleV4 {
	return []calicov3.BGPFilterRuleV4{
		{
			CIDR: allIPv4,
			// the match operator "In" matches all routes which are contained in the cidr,
			// therefore 0.0.0.0/0 matches every ipv4 route.
			MatchOperator: calicov3.MatchOperatorIn,
			Action:        calicov3.Reject,
		},
	}
}

// writeBGPConfiguration configures calico to advertise the ips of the cluster address pools. Calico
// only announces load balancer service ips which are contained in one of the specified cidr blocks.
// All other fields of the default BGPConfiguration are not touched, because they are maintained by
// the calico installation.
func (c *calicoConfig) writeBGPConfiguration(ctx context.Context) error {
	// the address pools are stored in a map, so the cidrs have to be sorted to get a deterministic result.
	var cidrs []string
	for _, pool := range c.base.AddressPools {
		cidrs = append(cidrs, pool.CIDRs...)
	}
	slices.Sort(cidrs)
	cidrs = slices.Compact(cidrs)

	blocks := make([]calicov3.ServiceLoadBalancerIPBlock, 0, len(cidrs))
	for _, cidr := range cidrs {
		blocks = append(blocks, calicov3.ServiceLoadBalancerIPBlock{CIDR: cidr})
	}

	bgpConfiguration := &calicov3.BGPConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: calicoGroupVersion.String(),
			Kind:       calicov3.KindBGPConfiguration,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: bgpConfigurationName,
		},
	}

	res, err := controllerutil.CreateOrUpdate(ctx, c.client, bgpConfiguration, func() error {
		bgpConfiguration.Spec.ServiceLoadBalancerIPs = blocks
		return nil
	})
	if err != nil {
		return err
	}

	if res != controllerutil.OperationResultNone {
		klog.Infof("bgpconfiguration: %v", res)
	}

	return nil
}

// writeBGPPeers writes one BGPPeer per node, which tells calico to peer with the bgp daemon (frr)
// running on the node itself. This requires calico >= v3.31, because the peering has to use the asn
// of the machine as local asn, which is only supported by BGPPeer.spec.localASNumber.
func (c *calicoConfig) writeBGPPeers(ctx context.Context) error {
	existingPeers := calicov3.BGPPeerList{}
	err := c.client.List(ctx, &existingPeers, client.MatchingLabels{managedByLabelKey: managedByLabelValue})
	if err != nil {
		return err
	}

	for i := range existingPeers.Items {
		existingPeer := existingPeers.Items[i]

		found := false
		for _, peer := range c.base.Peers {
			if bgpPeerName(peer) == existingPeer.Name {
				found = true
				break
			}
		}

		if !found {
			err := c.client.Delete(ctx, &existingPeer)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range c.base.Peers {
		bgpPeer := &calicov3.BGPPeer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: calicoGroupVersion.String(),
				Kind:       calicov3.KindBGPPeer,
			},
			ObjectMeta: metav1.ObjectMeta{
				// the name of the bgp peer is the asn of the metal-stack networking infrastructure
				// this node is peering with, this matches the peers of the other load balancer backends.
				Name: bgpPeerName(peer),
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, c.client, bgpPeer, func() error {
			bgpPeer.Labels = map[string]string{managedByLabelKey: managedByLabelValue}
			bgpPeer.Spec = calicov3.BGPPeerSpec{
				// this peer belongs to exactly this node, a node selector is not used on purpose.
				Node:   peer.NodeName,
				PeerIP: frrPeerAddress,
				// the asn the machine is announcing, both ends of the peering use the same asn.
				ASNumber: numorstring.ASNumber(peer.ASN),
				// the peering is an internal one, so the node uses the asn of the machine as local
				// asn as well. this field was added in calico v3.31, older installations prune it
				// and fall back to the asn of the BGPConfiguration (64512 by default). In this case
				// the peering with the frr daemon on the node is rejected and the load balancer ips
				// are not announced.
				LocalASNumber: new(numorstring.ASNumber(peer.MyASN)),
				// the peering is a purely local one, therefore no source address must be configured.
				SourceAddress: calicov3.SourceAddressNone,
				// restrict the peering to the load balancer ips, see writeBGPFilter.
				Filters: []string{bgpFilterName},
			}
			return nil
		})
		if err != nil {
			return err
		}

		if res != controllerutil.OperationResultNone {
			klog.Infof("bgppeer %q: %v", bgpPeer.Name, res)
		}
	}

	return nil
}

func bgpPeerName(peer *peer) string {
	return fmt.Sprintf("peer-%d", peer.ASN)
}
