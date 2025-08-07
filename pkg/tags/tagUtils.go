package tags

import (
	"fmt"
	"strings"

	t "github.com/metal-stack/metal-lib/pkg/tag"
)

// BuildClusterServiceFQNTag returns the ClusterServiceFQN tag populated with the given arguments.
func BuildClusterServiceFQNLabel(clusterID string, namespace, serviceName string) map[string]string {
	return map[string]string{
		t.ClusterServiceFQN: fmt.Sprintf("%s/%s/%s", clusterID, namespace, serviceName),
	}
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
