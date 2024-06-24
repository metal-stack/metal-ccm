package cilium

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-go/api/models"

	ciliumv2alpha1 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

type Cilium struct {
	Peers        []*Peer                     `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*loadbalancer.AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func NewCiliumConfig() *Cilium {
	return &Cilium{}
}

func (cfg *Cilium) Namespace() string {
	return ""
}

func (cfg *Cilium) CalculateConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error {
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

func (cfg *Cilium) computeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) error {
	for _, ip := range ips {
		if !nws.Has(*ip.Networkid) {
			klog.Infof("skipping ip %q: not part of cluster networks", *ip.Ipaddress)
			continue
		}
		net := *ip.Networkid
		cfg.addIPToPool(net, *ip)
	}
	return nil
}

func (cfg *Cilium) computePeers(nodes []v1.Node) error {
	cfg.Peers = []*Peer{} // we want an empty array of peers and not nil if there are no nodes
	for _, n := range nodes {
		labels := n.GetLabels()
		asnString, ok := labels[tag.MachineNetworkPrimaryASN]
		if !ok {
			return fmt.Errorf("node %q misses label: %s", n.GetName(), tag.MachineNetworkPrimaryASN)
		}
		asn, err := strconv.ParseInt(asnString, 10, 64)
		if err != nil {
			return fmt.Errorf("unable to parse valid integer from asn annotation: %w", err)
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

func (cfg *Cilium) getOrCreateAddressPool(poolName string) *loadbalancer.AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := loadbalancer.NewBGPAddressPool(poolName)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

func (cfg *Cilium) addIPToPool(network string, ip models.V1IPResponse) {
	t := ip.Type
	poolType := models.V1IPBaseTypeEphemeral
	if t != nil && *t == models.V1IPBaseTypeStatic {
		poolType = models.V1IPBaseTypeStatic
	}
	poolName := fmt.Sprintf("%s-%s", strings.ToLower(network), poolType)
	pool := cfg.getOrCreateAddressPool(poolName)
	pool.AppendIP(*ip.Ipaddress)
}

func (cfg *Cilium) ToYAML() (string, error) {
	bb, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}

func (cfg *Cilium) WriteCRs(ctx context.Context, c client.Client) error {
	err := cfg.writeCiliumBGPPeeringPolicies(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to write ciliumbgppeeringpolicy resources %w", err)
	}

	err = cfg.writeCiliumLoadBalancerIPPools(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to write ciliumloadbalancerippool resources %w", err)
	}

	return nil
}

func (cfg *Cilium) writeCiliumBGPPeeringPolicies(ctx context.Context, c client.Client) error {
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
				APIVersion: "cilium.io/v2alpha1",
				Kind:       "CiliumBGPPeeringPolicy",
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
						LocalASN:      peer.Peer.MyASN,
						ExportPodCIDR: pointer.Pointer(true),
						Neighbors: []ciliumv2alpha1.CiliumBGPNeighbor{
							{
								PeerAddress:     "127.0.0.1/32",
								PeerASN:         peer.Peer.ASN,
								GracefulRestart: &ciliumv2alpha1.CiliumBGPNeighborGracefulRestart{Enabled: true},
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
			klog.Infof("bgppeer: %v", res)
		}
	}

	return nil
}

func (cfg *Cilium) writeCiliumLoadBalancerIPPools(ctx context.Context, c client.Client) error {
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
				APIVersion: "cilium.io/v2alpha1",
				Kind:       "CiliumLoadBalancerIpPool",
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
				Cidrs: cidrs,
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
