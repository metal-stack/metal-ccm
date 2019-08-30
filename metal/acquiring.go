package metal

import (
	"fmt"

	"github.com/google/uuid"
	metalgo "github.com/metal-pod/metal-go"
)

const (
	prefix = "metallb-"
)

// AcquireIPs acquires given count of IPs within the given network.
func (r *ResourcesController) DeleteIPs(ips ...string) error {
	for _, ip := range ips {
		_, err := r.resources.client.IPDelete(ip)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ResourcesController) AcquireIPs(project, network string, count int) ([]string, error) {
	var ips []string
	for i := 0; i < count; i++ {
		name, err := uuid.NewUUID()
		if err != nil {
			return nil, err
		}

		req := &metalgo.IPAcquireRequest{
			Projectid: project,
			Networkid: network,
			Name:      fmt.Sprintf("%s%s", prefix, name.String()[:5]),
		}
		resp, err := r.resources.client.IPAcquire(req)
		if err != nil {
			return nil, err
		}
		ips = append(ips, *resp.IP.Ipaddress)
	}

	return ips, nil
}
