package loadbalancer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/metal-stack/metal-go/api/models"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LoadBalancerConfig interface {
	Namespace() string
	PrepareConfig(ips []*models.V1IPResponse, nws sets.Set[string], nodes []v1.Node) error
	WriteCRs(ctx context.Context, c client.Client) error
}

type Config struct {
	AddressPools []*AddressPool `json:"address-pools,omitempty" yaml:"address-pools,omitempty"`
}

func (cfg *Config) ComputeAddressPools(ips []*models.V1IPResponse, nws sets.Set[string]) error {
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

func (cfg *Config) addIPToPool(network string, ip models.V1IPResponse) error {
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

func (cfg *Config) getOrCreateAddressPool(poolName string) *AddressPool {
	for _, pool := range cfg.AddressPools {
		if pool.Name == poolName {
			return pool
		}
	}

	pool := NewBGPAddressPool(poolName)
	cfg.AddressPools = append(cfg.AddressPools, pool)

	return pool
}
