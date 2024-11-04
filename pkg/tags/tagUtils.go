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

// IsMemberOfCluster returns true of the given tag is a cluster-tag and clusterID matches.
// tag is in the form of:
//
//	cluster.metal-stack.io/id/namespace/service=<clusterid>/namespace/servicename
func IsMemberOfCluster(tag, clusterID string) bool {
	if strings.HasPrefix(tag, t.ClusterID) && strings.Contains(tag, "="+clusterID+"/") {
		return true
	}
	return false
}
