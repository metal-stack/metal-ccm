package metal

import (
	"fmt"
	"github.com/google/uuid"
	metalgo "github.com/metal-pod/metal-go"
	"strings"
)

// AcquireIPs acquires given count of IPs within the given network.
func (r *ResourcesController) AcquireIPs(project, network string, count int) (bool, error) {
	req := &metalgo.IPFindRequest{
		ProjectID: &project,
		NetworkID: &network,
	}
	resp, err := r.resources.client.IPFind(req)
	if err != nil {
		return false, err
	}

	if len(resp.IPs) >= count {
		return false, nil
	}

	ips := make([]string, count)
	for i, ip := range resp.IPs {
		if strings.Contains(ip.Name, prefix) {
			ips[i] = *ip.Ipaddress
		}
	}

	for i := len(resp.IPs); i < count; i++ {
		name, err := uuid.NewUUID()
		if err != nil {
			return false, err
		}

		req := &metalgo.IPAcquireRequest{
			Projectid: project,
			Networkid: network,
			Name:      fmt.Sprintf("%s%s", prefix, name.String()[:5]),
		}
		ip, err := r.resources.client.IPAcquire(req)
		if err != nil {
			return false, err
		}
		ips[i] = *ip.IP.Ipaddress
	}

	r.metallbConfig.announceIPs(network, ips...)

	return true, r.upsertMetalLBConfig()
}
