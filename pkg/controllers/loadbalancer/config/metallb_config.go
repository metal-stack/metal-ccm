package config

import (
	"context"
	"fmt"
	"time"

	"github.com/metal-stack/metal-lib/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
)

const (
	LoadBalancerTypeMetalLB LoadBalancerType = "metallb"

	metallbNamespace = "metallb-system"
)

type metalLBConfig struct {
	Base   *baseConfig
	client client.Client
}

func newMetalLBConfig(base *baseConfig, c client.Client) *metalLBConfig {
	return &metalLBConfig{Base: base, client: c}
}

func (m *metalLBConfig) WriteCRs(ctx context.Context) error {
	bgpPeerList := metallbv1beta2.BGPPeerList{}
	err := m.client.List(ctx, &bgpPeerList, client.InNamespace(metallbNamespace))
	if err != nil {
		return err
	}
	for _, existingPeer := range bgpPeerList.Items {
		existingPeer := existingPeer
		found := false

		for _, peer := range m.Base.Peers {
			if fmt.Sprintf("peer-%d", peer.ASN) == existingPeer.Name {
				found = true
				break
			}
		}

		if !found {
			err := m.client.Delete(ctx, &existingPeer)
			if err != nil {
				return err
			}
		}
	}

	for _, peer := range m.Base.Peers {
		bgpPeer := &metallbv1beta2.BGPPeer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta2",
				Kind:       "BGPPeer",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("peer-%d", peer.ASN),
				Namespace: metallbNamespace,
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, m.client, bgpPeer, func() error {
			bgpPeer.Spec = metallbv1beta2.BGPPeerSpec{
				MyASN:         peer.MyASN,
				ASN:           peer.ASN,
				HoldTime:      pointer.Pointer(metav1.Duration{Duration: 90 * time.Second}),
				KeepaliveTime: pointer.Pointer(metav1.Duration{Duration: 0 * time.Second}),
				Address:       peer.Address,
				NodeSelectors: []metav1.LabelSelector{peer.NodeSelector},
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

	addressPoolList := metallbv1beta1.IPAddressPoolList{}
	err = m.client.List(ctx, &addressPoolList, client.InNamespace(metallbNamespace))
	if err != nil {
		return err
	}
	for _, existingPool := range addressPoolList.Items {
		found := false
		for _, pool := range m.Base.AddressPools {
			if pool.Name == existingPool.Name {
				found = true
				break
			}
		}
		if !found {
			err := m.client.Delete(ctx, &existingPool)
			if err != nil {
				return err
			}
		}
	}

	for _, pool := range m.Base.AddressPools {
		ipAddressPool := &metallbv1beta1.IPAddressPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta1",
				Kind:       "IPAddressPool",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool.Name,
				Namespace: metallbNamespace,
			},
		}

		res, err := controllerutil.CreateOrUpdate(ctx, m.client, ipAddressPool, func() error {
			ipAddressPool.Spec = metallbv1beta1.IPAddressPoolSpec{
				Addresses:  pool.CIDRs,
				AutoAssign: pool.AutoAssign,
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

	for _, pool := range m.Base.AddressPools {
		bgpAdvertisementList := metallbv1beta1.BGPAdvertisementList{}
		err = m.client.List(ctx, &bgpAdvertisementList, client.InNamespace(metallbNamespace))
		if err != nil {
			return err
		}

		for _, existingAdvertisement := range bgpAdvertisementList.Items {
			existingAdvertisement := existingAdvertisement
			found := false
			for _, pool := range m.Base.AddressPools {
				if pool.Name == existingAdvertisement.Name {
					found = true
					break
				}
			}
			if !found {
				err := m.client.Delete(ctx, &existingAdvertisement)
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
		}

		res, err := controllerutil.CreateOrUpdate(ctx, m.client, bgpAdvertisement, func() error {
			bgpAdvertisement.Spec = metallbv1beta1.BGPAdvertisementSpec{
				IPAddressPools: []string{pool.Name},
			}
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
