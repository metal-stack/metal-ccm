package cilium

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"

	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-go/api/models"

	ciliumv2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

type ciliumConfig struct {
	Peers        []*Peer                     `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*loadbalancer.AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
	k8sClient    clientset.Interface
}

func NewCiliumConfig(k8sClient clientset.Interface) *ciliumConfig {
	return &ciliumConfig{k8sClient: k8sClient}
}

func (cfg *ciliumConfig) Namespace() string {
	return ""
}

func (cfg *ciliumConfig) PrepareConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error {
	err := cfg.computeAddressPools(ips, nws)
	if err != nil {
		return err
	}
	err = cfg.computePeers(nodes)
	if err != nil {
		return err
	}
	return nil
}

func (cfg *ciliumConfig) computeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) error {
	var errs []error
	for _, ip := range ips {
		if !nws.Has(*ip.Networkid) {
			klog.Infof("skipping ip %q: not part of cluster networks", *ip.Ipaddress)
			continue
		}
		net := *ip.Networkid
		err := cfg.addIPToPool(net, *ip)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (cfg *ciliumConfig) computePeers(nodes []v1.Node) error {
	cfg.Peers = []*Peer{} // we want an empty array of peers and not nil if there are no nodes
	for _, n := range nodes {
		asn, err := getASNFromNodeLabels(n)
		if err != nil {
			return err
		}

		peer, err := newPeer(n, asn)
		if err != nil {
			klog.Warningf("skipping peer: %v", err)
			continue
		}

		cfg.Peers = append(cfg.Peers, peer)
	}
	return nil
}

func (cfg *ciliumConfig) getOrCreateAddressPool(poolName string) *loadbalancer.AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := loadbalancer.NewBGPAddressPool(poolName)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

func (cfg *ciliumConfig) addIPToPool(network string, ip models.V1IPResponse) error {
	t := ip.Type
	poolType := models.V1IPBaseTypeEphemeral
	if t != nil && *t == models.V1IPBaseTypeStatic {
		poolType = models.V1IPBaseTypeStatic
	}
	poolName := fmt.Sprintf("%s-%s", strings.ToLower(network), poolType)
	pool := cfg.getOrCreateAddressPool(poolName)
	err := pool.AppendIP(*ip.Ipaddress)
	if err != nil {
		return err
	}
	return nil
}

func (cfg *ciliumConfig) toYAML() (string, error) {
	bb, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}

func (cfg *ciliumConfig) WriteCRs(ctx context.Context, c client.Client) error {
	err := cfg.writeCiliumBGPPeeringPolicies(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgppeeringpolicy resources %w", err)
	}

	err = cfg.writeCiliumLoadBalancerIPPools(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to write ciliumloadbalancerippool resources %w", err)
	}

	err = cfg.writeNodeAnnotations(ctx)
	if err != nil {
		return fmt.Errorf("failed to write node annotations %w", err)
	}

	return nil
}

func (cfg *ciliumConfig) writeCiliumBGPPeeringPolicies(ctx context.Context, c client.Client) error {
	existingPolicies := ciliumv2alpha1.CiliumBGPPeeringPolicyList{}
	err := c.List(ctx, &existingPolicies)
	if err != nil {
		return err
	}
	for _, existingPolicy := range existingPolicies.Items {
		existingPolicy := existingPolicy
		found := false
		for _, peer := range cfg.Peers {
			if fmt.Sprintf("%d", peer.Peer.ASN) == existingPolicy.Name {
				found = true
				break
			}
		}
		if !found {
			err := c.Delete(ctx, &existingPolicy)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range cfg.Peers {
		bgpPeeringPolicy := &ciliumv2alpha1.CiliumBGPPeeringPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2alpha1.CustomResourceDefinitionGroup + "/" + ciliumv2alpha1.CustomResourceDefinitionVersion,
				Kind:       ciliumv2alpha1.BGPPKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%d", peer.Peer.ASN),
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, bgpPeeringPolicy, func() error {
			bgpPeeringPolicy.Spec = ciliumv2alpha1.CiliumBGPPeeringPolicySpec{
				NodeSelector: &peer.NodeSelector,
				VirtualRouters: []ciliumv2alpha1.CiliumBGPVirtualRouter{
					{
						LocalASN:      int64(peer.Peer.MyASN),
						ExportPodCIDR: pointer.Pointer(true),
						Neighbors: []ciliumv2alpha1.CiliumBGPNeighbor{
							{
								PeerAddress:     "127.0.0.1/32",
								PeerASN:         int64(peer.Peer.ASN),
								GracefulRestart: &ciliumv2alpha1.CiliumBGPNeighborGracefulRestart{Enabled: true},
							},
						},
						// A NotIn match expression with a dummy key and value have to be used to announce ALL services.
						ServiceSelector: pointer.Pointer(slimv1.LabelSelector{
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

func (cfg *ciliumConfig) writeCiliumLoadBalancerIPPools(ctx context.Context, c client.Client) error {
	existingPools := ciliumv2alpha1.CiliumLoadBalancerIPPoolList{}
	err := c.List(ctx, &existingPools)
	if err != nil {
		return err
	}
	for _, existingPool := range existingPools.Items {
		existingPool := existingPool
		found := false
		for _, pool := range cfg.AddressPools {
			if pool.Name == existingPool.Name {
				found = true
				break
			}
		}
		if !found {
			err := c.Delete(ctx, &existingPool)
			if err != nil {
				return err
			}
		}
	}

	for _, pool := range cfg.AddressPools {
		ipPool := &ciliumv2alpha1.CiliumLoadBalancerIPPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: ciliumv2alpha1.CustomResourceDefinitionGroup + "/" + ciliumv2alpha1.CustomResourceDefinitionVersion,
				Kind:       ciliumv2alpha1.PoolKindDefinition,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: pool.Name,
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, ipPool, func() error {
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

func (cfg *ciliumConfig) writeNodeAnnotations(ctx context.Context) error {
	nodes, err := kubernetes.GetNodes(ctx, cfg.k8sClient)
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
		err = kubernetes.UpdateNodeAnnotationsWithBackoff(ctx, cfg.k8sClient, n.Name, annotations, backoff)
		if err != nil {
			return fmt.Errorf("failed to write node annotations for node %s: %w", n.Name, err)
		}
	}

	return nil
}

func getASNFromNodeLabels(node v1.Node) (int64, error) {
	labels := node.GetLabels()
	asnString, ok := labels[tag.MachineNetworkPrimaryASN]
	if !ok {
		return 0, fmt.Errorf("node %q misses label: %s", node.GetName(), tag.MachineNetworkPrimaryASN)
	}
	asn, err := strconv.ParseInt(asnString, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse valid integer from asn annotation: %w", err)
	}
	return asn, nil
}
