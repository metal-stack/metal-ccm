package config

import (
	"context"
	"fmt"
	"slices"

	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	"github.com/projectcalico/api/pkg/lib/numorstring"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	LoadBalancerTypeCalico LoadBalancerType = "calico"

	// bgpConfigurationName is the name of the default BGPConfiguration of a calico installation.
	// Calico has only this one cluster wide default BGPConfiguration, so the load balancer ips have
	// to be configured in this resource.
	bgpConfigurationName = "default"

	// frrPeerAddress is the address of the bgp daemon (frr) running on the node itself. Calico peers
	// with this address to announce the load balancer ips to the metal-stack networking infrastructure.
	frrPeerAddress = "127.0.0.1"

	// managedByLabelKey and managedByLabelValue mark the BGPPeer resources which are maintained by
	// metal-ccm. Peerings which are not labeled this way are managed by someone else and are left
	// untouched by metal-ccm.
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "metal-ccm"
)

type calicoConfig struct {
	base   *baseConfig
	client client.Client
}

func newCalicoConfig(base *baseConfig, c client.Client) *calicoConfig {
	return &calicoConfig{base: base, client: c}
}

func (c *calicoConfig) WriteCRs(ctx context.Context) error {
	err := c.writeBGPConfiguration(ctx)
	if err != nil {
		return fmt.Errorf("failed to write bgpconfiguration resource %w", err)
	}

	err = c.writeBGPPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to write bgppeer resources %w", err)
	}

	return nil
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
			APIVersion: calicov3.GroupVersionCurrent,
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
// running on the node itself.
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
				APIVersion: calicov3.GroupVersionCurrent,
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
				ASNumber:      numorstring.ASNumber(peer.ASN),
				LocalASNumber: new(numorstring.ASNumber(peer.MyASN)),
				// the peering is a purely local one, therefore no source address must be configured.
				SourceAddress: calicov3.SourceAddressNone,
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
