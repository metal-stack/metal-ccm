package tags

import (
	"fmt"
	t "github.com/metal-stack/metal-lib/pkg/tag"
	"strings"
)

// BuildClusterServiceFQNTag returns the ClusterServiceFQN tag populated with the given arguments.
func BuildClusterServiceFQNTag(clusterID string, namespace, serviceName string) string {
	return fmt.Sprintf("%s=%s/%s/%s", t.ClusterServiceFQN, clusterID, namespace, serviceName)
}

// IsMachine returns true if the given tag is a machine-tag.
func IsMachine(tag string) bool {
	return strings.HasPrefix(tag, t.MachineID)
}

// IsMemberOfCluster returns true of the given tag is a cluster-tag and clusterID matches.
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

