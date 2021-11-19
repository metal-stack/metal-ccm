package tags

import (
	"testing"

	"github.com/metal-stack/metal-lib/pkg/tag"
)

func TestIsMemberOfCluster(t *testing.T) {
	tests := []struct {
		name      string
		tag       string
		clusterID string
		want      bool
	}{
		{
			name:      "valid cluster service tag on ip",
			tag:       "cluster.metal-stack.io/id/namespace/service=e0ab89d8-c087-4c5a-9e86-7656a2371c24/default/echoserver-ext",
			clusterID: "e0ab89d8-c087-4c5a-9e86-7656a2371c24",
			want:      true,
		},
		{
			name:      "valid cluster pod tag on ip",
			tag:       tag.ClusterID + "/id/namespace/pod=e0ab89d8-c087-4c5a-9e86-7656a2371c24/default/echoserver-ext",
			clusterID: "e0ab89d8-c087-4c5a-9e86-7656a2371c24",
			want:      true,
		},
		{
			name:      "invalid cluster service tag on ip",
			tag:       "blubber=e0ab89d8-c087-4c5a-9e86-7656a2371c24/default/echoserver-ext",
			clusterID: "e0ab89d8-c087-4c5a-9e86-7656a2371c24",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMemberOfCluster(tt.tag, tt.clusterID); got != tt.want {
				t.Errorf("IsMemberOfCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}
