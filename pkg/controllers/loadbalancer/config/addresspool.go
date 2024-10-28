package config

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

const (
	bgpProtocol = "bgp"
)

type addressPool struct {
	Name       string
	Protocol   string
	AutoAssign *bool
	CIDRs      []string // It is assumed that only host addresses (/32 for ipv4 or /128 for ipv6) are used.
}

type addressPools []addressPool

func newBGPAddressPool(name string) addressPool {
	return addressPool{
		Name:       name,
		Protocol:   bgpProtocol,
		AutoAssign: pointer.Pointer(false),
	}
}

func (pool *addressPool) appendIP(ip *models.V1IPResponse) error {
	if ip.Ipaddress == nil {
		return errors.New("ip address is not set on ip")
	}

	parsed, err := netip.ParseAddr(*ip.Ipaddress)
	if err != nil {
		return err
	}

	cidr := fmt.Sprintf("%s/%d", parsed.String(), parsed.BitLen())

	if slices.ContainsFunc(pool.CIDRs, func(elem string) bool {
		return cidr == elem
	}) {
		return nil
	}

	pool.CIDRs = append(pool.CIDRs, cidr)

	return nil
}

func (as addressPools) addPoolIP(poolName string, ip *models.V1IPResponse) (addressPools, error) {
	idx := slices.IndexFunc(as, func(a addressPool) bool {
		return a.Name == poolName
	})

	if idx < 0 {
		as = append(as, newBGPAddressPool(poolName))
		idx = 0
	}

	err := as[idx].appendIP(ip)
	if err != nil {
		return nil, err
	}

	return as, nil
}

func getPoolName(network string, ip *models.V1IPResponse) string {
	poolType := models.V1IPBaseTypeEphemeral
	if pointer.SafeDeref(ip.Type) == models.V1IPBaseTypeStatic {
		poolType = models.V1IPBaseTypeStatic
	}

	return fmt.Sprintf("%s-%s", strings.ToLower(network), poolType)
}
