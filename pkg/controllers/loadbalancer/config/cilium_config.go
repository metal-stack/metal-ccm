package config

import (
	"context"
	"fmt"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	LoadBalancerTypeCilium LoadBalancerType = "cilium"

	// the BGP peer config and the BGP advertisements are cluster wide singleton resources,
	// every ciliumbgpclusterconfig we create refers to them.
	peerConfigName    = "metal-ccm-peer-config"
	advertisementName = "metal-ccm-advertisements"

	// the advertisements are labeled with this label, the peer config selects them with a label selector.
	advertisementLabelKey   = "advertise"
	advertisementLabelValue = "bgp"
)

type ciliumConfig struct {
	base   *baseConfig
	client client.Client
}

func newCiliumConfig(base *baseConfig, c client.Client) *ciliumConfig {
	return &ciliumConfig{base: base, client: c}
}

func (c *ciliumConfig) WriteCRs(ctx context.Context) error {
	err := c.writeCiliumBGPPeerConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgppeerconfig resources %w", err)
	}

	err = c.writeCiliumBGPAdvertisements(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgpadvertisement resources %w", err)
	}

	err = c.writeCiliumBGPClusterConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgpclusterconfig resources %w", err)
	}

	err = c.writeCiliumBGPNodeConfigOverrides(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgpnodeconfigoverride resources %w", err)
	}

	err = c.writeCiliumLoadBalancerIPPools(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumloadbalancerippool resources %w", err)
	}

	return nil
}

// writeCiliumBGPPeerConfig writes the cluster wide BGP peer config which is referenced by all BGP
// cluster configs of this cluster.
func (c *ciliumConfig) writeCiliumBGPPeerConfig(ctx context.Context) error {
	peerConfig := &ciliumv2.CiliumBGPPeerConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ciliumv2.CustomResourceDefinitionGroup + "/" + ciliumv2.CustomResourceDefinitionVersion,
			Kind:       ciliumv2.BGPPCKindDefinition,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: peerConfigName,
		},
	}

	res, err := controllerutil.CreateOrUpdate(ctx, c.client, peerConfig, func() error {
		peerConfig.Spec = ciliumv2.CiliumBGPPeerConfigSpec{
			GracefulRestart: &ciliumv2.CiliumBGPNeighborGracefulRestart{
				Enabled: true,
			},
			Families: []ciliumv2.CiliumBGPFamilyWithAdverts{
				{
					CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
					Advertisements: &slimv1.LabelSelector{
						MatchLabels: map[string]string{advertisementLabelKey: advertisementLabelValue},
					},
				},
				{
					CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv6", Safi: "unicast"},
					Advertisements: &slimv1.LabelSelector{
						MatchLabels: map[string]string{advertisementLabelKey: advertisementLabelValue},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}

	if res != controllerutil.OperationResultNone {
		klog.Infof("bgppeerconfig: %v", res)
	}

	return nil
}

// writeCiliumBGPAdvertisements writes the cluster wide BGP advertisements which are referenced by the
// BGP peer config of this cluster.
func (c *ciliumConfig) writeCiliumBGPAdvertisements(ctx context.Context) error {
	advertisements := &ciliumv2.CiliumBGPAdvertisement{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ciliumv2.CustomResourceDefinitionGroup + "/" + ciliumv2.CustomResourceDefinitionVersion,
			Kind:       ciliumv2.BGPAKindDefinition,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   advertisementName,
			Labels: map[string]string{advertisementLabelKey: advertisementLabelValue},
		},
	}

	res, err := controllerutil.CreateOrUpdate(ctx, c.client, advertisements, func() error {
		advertisements.Labels = map[string]string{advertisementLabelKey: advertisementLabelValue}
		advertisements.Spec = ciliumv2.CiliumBGPAdvertisementSpec{
			Advertisements: []ciliumv2.BGPAdvertisement{
				// this replaces the exportPodCIDR option of a bgp control plane v1 virtual router.
				{
					AdvertisementType: ciliumv2.BGPPodCIDRAdvert,
				},
				// A NotIn match expression with a dummy key and value have to be used to announce ALL services.
				{
					AdvertisementType: ciliumv2.BGPServiceAdvert,
					Service: &ciliumv2.BGPServiceOptions{
						Addresses: []ciliumv2.BGPServiceAddressType{ciliumv2.BGPLoadBalancerIPAddr},
					},
					Selector: serviceSelector(),
				},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}

	if res != controllerutil.OperationResultNone {
		klog.Infof("bgpadvertisement: %v", res)
	}

	return nil
}

// writeCiliumBGPClusterConfigs writes one BGP cluster config per peer, which selects the nodes that
// should peer with the metal-stack networking infrastructure.
func (c *ciliumConfig) writeCiliumBGPClusterConfigs(ctx context.Context) error {
	existingConfigs := ciliumv2.CiliumBGPClusterConfigList{}
	err := c.client.List(ctx, &existingConfigs)
	if err != nil {
		return err
	}

	for _, existingConfig := range existingConfigs.Items {
		found := false

		for _, peer := range c.base.Peers {
			if fmt.Sprintf("%d", peer.ASN) == existingConfig.Name {
				found = true
				break
			}
		}

		if !found {
			err := c.client.Delete(ctx, &existingConfig)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range c.base.Peers {
		bgpClusterConfig := &ciliumv2.CiliumBGPClusterConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2.CustomResourceDefinitionGroup + "/" + ciliumv2.CustomResourceDefinitionVersion,
				Kind:       ciliumv2.BGPCCKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				// the name of a bgp cluster config is the ASN of the metal-stack networking infrastructure
				// the selected nodes are peering with.
				Name: fmt.Sprintf("%d", peer.ASN),
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, c.client, bgpClusterConfig, func() error {
			bgpClusterConfig.Spec = ciliumv2.CiliumBGPClusterConfigSpec{
				NodeSelector: convertNodeSelector(&peer.NodeSelector),
				BGPInstances: []ciliumv2.CiliumBGPInstance{
					{
						// the name of the instance has to match the name used in the bgp node config override of the node.
						Name:     fmt.Sprintf("instance-%d", peer.ASN),
						LocalASN: new(int64(peer.MyASN)),
						Peers: []ciliumv2.CiliumBGPPeer{
							{
								Name: fmt.Sprintf("peer-%d", peer.ASN),
								// the bgp daemon running on the node itself listens here.
								PeerAddress: new("127.0.0.1"),
								PeerASN:     new(int64(peer.ASN)),
								PeerConfigRef: &ciliumv2.PeerConfigReference{
									Name: peerConfigName,
								},
							},
						},
					},
				},
			}
			return nil
		})
		if err != nil {
			return err
		}

		if res != controllerutil.OperationResultNone {
			klog.Infof("bgpclusterconfig: %v", res)
		}
	}

	return nil
}

func (c *ciliumConfig) writeCiliumLoadBalancerIPPools(ctx context.Context) error {
	existingPools := ciliumv2.CiliumLoadBalancerIPPoolList{}
	err := c.client.List(ctx, &existingPools)
	if err != nil {
		return err
	}

	for _, existingPool := range existingPools.Items {
		found := false

		for _, pool := range c.base.AddressPools {
			if pool.Name == existingPool.Name {
				found = true
				break
			}
		}

		if !found {
			err := c.client.Delete(ctx, &existingPool)
			if err != nil {
				return err
			}
		}
	}

	for _, pool := range c.base.AddressPools {
		ipPool := &ciliumv2.CiliumLoadBalancerIPPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2.CustomResourceDefinitionGroup + "/" + ciliumv2.CustomResourceDefinitionVersion,
				Kind:       ciliumv2.PoolKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: pool.Name,
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, c.client, ipPool, func() error {
			cidrs := make([]ciliumv2.CiliumLoadBalancerIPPoolIPBlock, 0)
			for _, cidr := range pool.CIDRs {
				ipPoolBlock := ciliumv2.CiliumLoadBalancerIPPoolIPBlock{
					Cidr: ciliumv2.IPv4orIPv6CIDR(cidr),
				}
				cidrs = append(cidrs, ipPoolBlock)
			}
			ipPool.Spec = ciliumv2.CiliumLoadBalancerIPPoolSpec{
				Blocks: cidrs,
			}
			return nil
		})
		if err != nil {
			return err
		}

		if res != controllerutil.OperationResultNone {
			klog.Infof("ipaddresspool: %v", res)
		}
	}

	return nil
}

// writeCiliumBGPNodeConfigOverrides writes one BGP node config override per peer, which sets the router
// id of the BGP instance running on this node. This replaces the bgp control plane v1 virtual router
// node annotations, which are not evaluated by cilium >= 1.19 anymore. They are therefore not cleaned up
// on already existing nodes, they are simply gone as soon as a node is rolled.
func (c *ciliumConfig) writeCiliumBGPNodeConfigOverrides(ctx context.Context) error {
	existingOverrides := ciliumv2.CiliumBGPNodeConfigOverrideList{}
	err := c.client.List(ctx, &existingOverrides)
	if err != nil {
		return err
	}

	for _, existingOverride := range existingOverrides.Items {
		found := false

		for _, peer := range c.base.Peers {
			if peer.NodeName == existingOverride.Name {
				found = true
				break
			}
		}

		if !found {
			err := c.client.Delete(ctx, &existingOverride)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range c.base.Peers {
		nodeConfigOverride := &ciliumv2.CiliumBGPNodeConfigOverride{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2.CustomResourceDefinitionGroup + "/" + ciliumv2.CustomResourceDefinitionVersion,
				Kind:       ciliumv2.BGPNCOKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				// the name of a ciliumbgpnodeconfigoverride must match the name of the node it belongs to.
				Name: peer.NodeName,
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, c.client, nodeConfigOverride, func() error {
			nodeConfigOverride.Spec = ciliumv2.CiliumBGPNodeConfigOverrideSpec{
				BGPInstances: []ciliumv2.CiliumBGPNodeConfigInstanceOverride{
					{
						Name: fmt.Sprintf("instance-%d", peer.ASN),
						// the router id of the bgp instance running on this node.
						RouterID: new("127.0.0.1"),
					},
				},
			}
			return nil
		})
		if err != nil {
			return err
		}

		if res != controllerutil.OperationResultNone {
			klog.Infof("bgpnodeconfigoverride: %v", res)
		}
	}

	return nil
}

func serviceSelector() *slimv1.LabelSelector {
	// A NotIn match expression with a dummy key and value have to be used to announce ALL services.
	return &slimv1.LabelSelector{
		MatchExpressions: []slimv1.LabelSelectorRequirement{
			{
				Key:      ciliumv2.BGPLoadBalancerClass,
				Operator: slimv1.LabelSelectorOpNotIn,
				Values:   []string{"ignore"},
			},
		},
	}
}

func convertNodeSelector(s *metav1.LabelSelector) *slimv1.LabelSelector {
	var machExpressions []slimv1.LabelSelectorRequirement
	for _, me := range s.MatchExpressions {
		machExpressions = append(machExpressions, slimv1.LabelSelectorRequirement{
			Key:      me.Key,
			Operator: slimv1.LabelSelectorOperator(me.Operator),
			Values:   me.Values,
		})
	}
	return &slimv1.LabelSelector{
		MatchExpressions: machExpressions,
	}
}
