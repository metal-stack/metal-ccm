package tags

import (
	"fmt"
	"strings"

	t "github.com/metal-stack/metal-lib/pkg/tag"
)

// BuildClusterServiceFQNTag returns the ClusterServiceFQN tag populated with the given arguments.
func BuildClusterServiceFQNTag(clusterID string, namespace, serviceName string) string {
	return fmt.Sprintf("%s=%s/%s/%s", t.ClusterServiceFQN, clusterID, namespace, serviceName)
}

// IsMachine returns true if the given tag is a machine-tag.
func IsMachine(tag string) bool {
	return strings.HasPrefix(tag, t.MachineID)
}

// IsEgress returns true if the given tag is an egress-tag
func IsEgress(tag string) bool {
	return strings.HasPrefix(tag, t.ClusterEgress)
}

// IsMemberOfCluster returns true of the given tag is a cluster-tag and clusterID matches.
// tag is in the form of:
//
//	cluster.metal-stack.io/id/namespace/service=<clusterid>/namespace/servicename
func IsMemberOfCluster(tag, clusterID string) bool {
	if strings.HasPrefix(tag, t.ClusterID) {
		parts := strings.Split(tag, "=")
		if len(parts) != 2 {
			return false
		}
		if strings.HasPrefix(parts[1], clusterID) {
			return true
		}
	}
	return false
}

func GetMachineClusterTag(tags []string) (string, bool) {
	found := false
	value := ""
	for _, tag := range tags {
		if strings.HasPrefix(tag, t.ClusterID) {
			_, value, found = strings.Cut(tag, "=")
			break
		}
	}
	return value, found
}
