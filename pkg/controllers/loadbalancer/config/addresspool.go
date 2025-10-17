package config

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/metal-stack/api/go/enum"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
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

type addressPools map[string]addressPool

func newBGPAddressPool(name string) addressPool {
	return addressPool{
		Name:       name,
		Protocol:   bgpProtocol,
		AutoAssign: pointer.Pointer(false),
	}
}

func (pool *addressPool) appendIP(ip *apiv2.IP) error {
	if ip.Ip == "" {
		return errors.New("ip address is not set on ip")
	}

	parsed, err := netip.ParseAddr(ip.Ip)
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

func (as addressPools) addPoolIP(poolName string, ip *apiv2.IP) error {

	pool, ok := as[poolName]
	if !ok {
		as[poolName] = newBGPAddressPool(poolName)
		pool = as[poolName]
	}

	err := pool.appendIP(ip)
	if err != nil {
		return err
	}
	as[poolName] = pool
	return nil
}

func getPoolName(network string, ip *apiv2.IP) (string, error) {
	poolType := apiv2.IPType_IP_TYPE_EPHEMERAL
	if ip.Type == apiv2.IPType_IP_TYPE_STATIC {
		poolType = apiv2.IPType_IP_TYPE_STATIC
	}

	poolTypeString, err := enum.GetStringValue(poolType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", strings.ToLower(network), *poolTypeString), nil
}
