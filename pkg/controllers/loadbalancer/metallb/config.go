package metallb

import (
	"context"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/metal-stack/metal-ccm/pkg/controllers/loadbalancer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
)

const (
	metallbNamespace = "metallb-system"
)

type metalLBConfig struct {
	loadbalancer.Config
	namespace string
}

func NewMetalLBConfig() *metalLBConfig {
	return &metalLBConfig{namespace: metallbNamespace}
}

func (cfg *metalLBConfig) WriteCRs(ctx context.Context, c client.Client) error {
	bgpPeerList := metallbv1beta2.BGPPeerList{}
	err := c.List(ctx, &bgpPeerList, client.InNamespace(cfg.namespace))
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
		bgpPeer := &metallbv1beta2.BGPPeer{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta2",
				Kind:       "BGPPeer",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("peer-%d", peer.ASN),
				Namespace: cfg.namespace,
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, bgpPeer, func() error {
			bgpPeer.Spec = metallbv1beta2.BGPPeerSpec{
				MyASN:         peer.MyASN,
				ASN:           peer.ASN,
				HoldTime:      metav1.Duration{Duration: 90 * time.Second},
				KeepaliveTime: metav1.Duration{Duration: 0 * time.Second},
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
	err = c.List(ctx, &addressPoolList, client.InNamespace(cfg.namespace))
	if err != nil {
		return err
	}
	for _, existingPool := range addressPoolList.Items {
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
		ipAddressPool := &metallbv1beta1.IPAddressPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "metallb.io/v1beta1",
				Kind:       "IPAddressPool",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool.Name,
				Namespace: cfg.namespace,
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, ipAddressPool, func() error {
			ipAddressPool.Spec = metallbv1beta1.IPAddressPoolSpec{
				Addresses:  pool.CIDRs,
				AutoAssign: pool.AutoAssign,
			}

			klog.Info("writing address pool")
			spew.Dump(ipAddressPool.Spec)

			return nil
		})
		if err != nil {
			return err
		}
		if res != controllerutil.OperationResultNone {
			klog.Infof("ipaddresspool: %v", res)
		}
	}

	for _, pool := range cfg.AddressPools {
		bgpAdvertisementList := metallbv1beta1.BGPAdvertisementList{}
		err = c.List(ctx, &bgpAdvertisementList, client.InNamespace(cfg.namespace))
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
				Namespace: cfg.namespace,
			},
		}
		res, err := controllerutil.CreateOrUpdate(ctx, c, bgpAdvertisement, func() error {
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
