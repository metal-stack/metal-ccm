package config

import (
	"context"
	"fmt"
	"time"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"

	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	ciliumv2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	LoadBalancerTypeCilium LoadBalancerType = "cilium"
)

type ciliumConfig struct {
	base      *baseConfig
	client    client.Client
	k8sClient clientset.Interface
}

func newCiliumConfig(base *baseConfig, c client.Client, k8sClient clientset.Interface) *ciliumConfig {
	return &ciliumConfig{base: base, client: c, k8sClient: k8sClient}
}

func (c *ciliumConfig) WriteCRs(ctx context.Context) error {
	err := c.writeCiliumBGPPeeringPolicies(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgppeeringpolicy resources %w", err)
	}

	err = c.writeCiliumLoadBalancerIPPools(ctx)
	if err != nil {
		return fmt.Errorf("failed to write ciliumloadbalancerippool resources %w", err)
	}

	err = c.writeNodeAnnotations(ctx)
	if err != nil {
		return fmt.Errorf("failed to write node annotations %w", err)
	}

	return nil
}

func (c *ciliumConfig) writeCiliumBGPPeeringPolicies(ctx context.Context) error {
	existingPolicies := ciliumv2alpha1.CiliumBGPPeeringPolicyList{}
	err := c.client.List(ctx, &existingPolicies)
	if err != nil {
		return err
	}

	for _, existingPolicy := range existingPolicies.Items {
		found := false

		for _, peer := range c.base.Peers {
			if fmt.Sprintf("%d", peer.ASN) == existingPolicy.Name {
				found = true
				break
			}
		}

		if !found {
			err := c.client.Delete(ctx, &existingPolicy)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range c.base.Peers {
		bgpPeeringPolicy := &ciliumv2alpha1.CiliumBGPPeeringPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2alpha1.CustomResourceDefinitionGroup + "/" + ciliumv2alpha1.CustomResourceDefinitionVersion,
				Kind:       ciliumv2alpha1.BGPPKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%d", peer.ASN),
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, c.client, bgpPeeringPolicy, func() error {
			bgpPeeringPolicy.Spec = ciliumv2alpha1.CiliumBGPPeeringPolicySpec{
				NodeSelector: convertNodeSelector(&peer.NodeSelector),
				VirtualRouters: []ciliumv2alpha1.CiliumBGPVirtualRouter{
					{
						LocalASN:      int64(peer.MyASN),
						ExportPodCIDR: new(true),
						Neighbors: []ciliumv2alpha1.CiliumBGPNeighbor{
							{
								PeerAddress:     "127.0.0.1/32",
								PeerASN:         int64(peer.ASN),
								GracefulRestart: &ciliumv2alpha1.CiliumBGPNeighborGracefulRestart{Enabled: true},
							},
						},
						// A NotIn match expression with a dummy key and value have to be used to announce ALL services.
						ServiceSelector: new(slimv1.LabelSelector{
							MatchExpressions: []slimv1.LabelSelectorRequirement{
								{
									Key:      ciliumv2alpha1.BGPLoadBalancerClass,
									Operator: slimv1.LabelSelectorOpNotIn,
									Values:   []string{"ignore"},
								},
							},
						}),
					},
				},
			}
			return nil
		})
		if err != nil {
			return err
		}

		if res != controllerutil.OperationResultNone {
			klog.Infof("bgppeer: %v", res)
		}
	}

	return nil
}

func (c *ciliumConfig) writeCiliumLoadBalancerIPPools(ctx context.Context) error {
	existingPools := ciliumv2alpha1.CiliumLoadBalancerIPPoolList{}
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
		ipPool := &ciliumv2alpha1.CiliumLoadBalancerIPPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2alpha1.CustomResourceDefinitionGroup + "/" + ciliumv2alpha1.CustomResourceDefinitionVersion,
				Kind:       ciliumv2alpha1.PoolKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: pool.Name,
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, c.client, ipPool, func() error {
			cidrs := make([]ciliumv2alpha1.CiliumLoadBalancerIPPoolIPBlock, 0)
			for _, cidr := range pool.CIDRs {
				ipPoolBlock := ciliumv2alpha1.CiliumLoadBalancerIPPoolIPBlock{
					Cidr: ciliumv2alpha1.IPv4orIPv6CIDR(cidr),
				}
				cidrs = append(cidrs, ipPoolBlock)
			}
			ipPool.Spec = ciliumv2alpha1.CiliumLoadBalancerIPPoolSpec{
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

func (c *ciliumConfig) writeNodeAnnotations(ctx context.Context) error {
	nodes, err := kubernetes.GetNodes(ctx, c.k8sClient)
	if err != nil {
		return fmt.Errorf("failed to write node annotations: %w", err)
	}

	backoff := wait.Backoff{
		Steps:    20,
		Duration: 50 * time.Millisecond,
		Jitter:   1.0,
	}

	for _, n := range nodes {
		asn, err := getASNFromNodeLabels(n)
		if err != nil {
			return fmt.Errorf("failed to write node annotations for node %s: %w", n.Name, err)
		}

		annotations := map[string]string{
			fmt.Sprintf("cilium.io/bgp-virtual-router.%d", asn): "router-id=127.0.0.1",
		}

		err = kubernetes.UpdateNodeAnnotationsWithBackoff(ctx, c.k8sClient, n.Name, annotations, backoff)
		if err != nil {
			return fmt.Errorf("failed to write node annotations for node %s: %w", n.Name, err)
		}
	}

	return nil
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
