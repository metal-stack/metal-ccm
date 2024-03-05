package loadbalancer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/metal-stack/metal-ccm/pkg/resources/kubernetes"
	"github.com/metal-stack/metal-lib/pkg/tag"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/metal-stack/metal-go/api/models"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
)

const (
	metallbNamespace     = "metallb-system"
	metallbConfigMapName = "config"
	metallbConfigMapKey  = "config"
)

// MetalLBConfig is a struct containing a config for metallb
type MetalLBConfig struct {
	Peers        []*Peer        `json:"peers,omitempty" yaml:"peers,omitempty"`
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func newMetalLBConfig() *MetalLBConfig {
	return &MetalLBConfig{}
}

// CalculateConfig computes the metallb config from given parameter input.
func (cfg *MetalLBConfig) CalculateConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error {
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

func (cfg *MetalLBConfig) computeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) error {
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

func (cfg *MetalLBConfig) computePeers(nodes []v1.Node) error {
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

// Write inserts or updates the Metal-LB config.
func (cfg *MetalLBConfig) Write(ctx context.Context, client clientset.Interface) error {
	yaml, err := cfg.ToYAML()
	if err != nil {
		return err
	}

	cm := make(map[string]string, 1)
	cm[metallbConfigMapKey] = yaml

	return kubernetes.ApplyConfigMap(ctx, client, metallbNamespace, metallbConfigMapName, cm)
}

// getOrCreateAddressPool returns the address pool of the given network.
// It will be created if it does not exist yet.
func (cfg *MetalLBConfig) getOrCreateAddressPool(poolName string) *AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := NewBGPAddressPool(poolName)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}

// announceIPs appends the given IPs to the network address pools.
func (cfg *MetalLBConfig) addIPToPool(network string, ip models.V1IPResponse) {
	t := ip.Type
	poolType := models.V1IPBaseTypeEphemeral
	if t != nil && *t == models.V1IPBaseTypeStatic {
		poolType = models.V1IPBaseTypeStatic
	}
	poolName := fmt.Sprintf("%s-%s", network, poolType)
	pool := cfg.getOrCreateAddressPool(poolName)
	pool.appendIP(*ip.Ipaddress)
}

// ToYAML returns this config in YAML format.
func (cfg *MetalLBConfig) ToYAML() (string, error) {
	bb, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}

// Write inserts or updates the Metal-LB custom resources.
func (cfg *MetalLBConfig) WriteCRs(ctx context.Context, c client.Client) error {

	// BGPPeers
	bgpPeerList := metallbv1beta2.BGPPeerList{}
	err := c.List(ctx, &bgpPeerList, client.InNamespace(metallbNamespace))
	if err != nil {
		return err
	}
	for _, existingPeer := range bgpPeerList.Items {
		existingPeer := existingPeer
		found := false
		for _, peer := range cfg.Peers {
			if fmt.Sprintf("peer-%d", peer.ASN) == existingPeer.Name {
				found = true
				break
			}
		}
		if !found {
			err := c.Delete(ctx, &existingPeer)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range cfg.Peers {
		bgpPeer := &metallbv1beta2.BGPPeer{ // Add '&' to pass a pointer to BGPPeer
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta2",
				Kind:       "BGPPeer",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("peer-%d", peer.ASN),
				Namespace: metallbNamespace,
			},
			Spec: metallbv1beta2.BGPPeerSpec{
				MyASN:         uint32(peer.MyASN),
				ASN:           uint32(peer.ASN),
				HoldTime:      metav1.Duration{Duration: 90 * time.Second},
				KeepaliveTime: metav1.Duration{Duration: 0 * time.Second},
				Address:       peer.Address,
				NodeSelectors: []metav1.LabelSelector{{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      peer.NodeSelectors[0].MatchExpressions[0].Key,
						Operator: metav1.LabelSelectorOpIn,
						Values:   peer.NodeSelectors[0].MatchExpressions[0].Values,
					}},
				}},
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, bgpPeer, func() error {
			return nil
		})
		if err != nil {
			return err
		}
		if res != controllerutil.OperationResultNone {
			klog.Infof("bgppeer: %v", res)
		}
	}

	// IPAddressPools
	addressPoolList := metallbv1beta1.IPAddressPoolList{}
	err = c.List(ctx, &addressPoolList, client.InNamespace(metallbNamespace))
	if err != nil {
		return err
	}
	for _, existingPool := range addressPoolList.Items {
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
		pool := &metallbv1beta1.IPAddressPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta1",
				Kind:       "IPAddressPool",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool.Name,
				Namespace: metallbNamespace,
			},
			Spec: metallbv1beta1.IPAddressPoolSpec{
				Addresses:  pool.CIDRs,
				AutoAssign: pool.AutoAssign,
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, pool, func() error {
			return nil
		})
		if err != nil {
			return err
		}
		if res != controllerutil.OperationResultNone {
			klog.Infof("ipaddresspool: %v", res)
		}
	}

	// BGPAdvertisements
	for _, pool := range cfg.AddressPools {
		bgpAdvertisementList := metallbv1beta1.BGPAdvertisementList{}
		err = c.List(ctx, &bgpAdvertisementList, client.InNamespace(metallbNamespace))
		if err != nil {
			return err
		}
		for _, existingAdvertisement := range bgpAdvertisementList.Items {
			existingAdvertisement := existingAdvertisement
			found := false
			for _, pool := range cfg.AddressPools {
				if pool.Name == existingAdvertisement.Name {
					found = true
					break
				}
			}
			if !found {
				err := c.Delete(ctx, &existingAdvertisement)
				if err != nil {
					return err
				}
			}
		}

		bgpAdvertisement := &metallbv1beta1.BGPAdvertisement{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta1",
				Kind:       "BGPAdvertisement",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool.Name,
				Namespace: metallbNamespace,
			},
			Spec: metallbv1beta1.BGPAdvertisementSpec{
				IPAddressPools: []string{pool.Name},
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, bgpAdvertisement, func() error {
			return nil
		})
		if err != nil {
			return err
		}
		if res != controllerutil.OperationResultNone {
			klog.Infof("bgpadvertisement: %v", res)
		}
	}

	return nil
}
