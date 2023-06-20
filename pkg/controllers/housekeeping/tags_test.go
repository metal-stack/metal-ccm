package housekeeping

import "testing"

func Test_buildLabelsFromMachineTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want map[string]string
	}{
		{
			name: "basic tags should work",
			tags: []string{"partition=apartition", "machine=amachineid"},
			want: map[string]string{"machine": "amachineid", "partition": "apartition"},
		},
		{
			name: "label without value should be ignored",
			tags: []string{"partition=apartition", "machine=amachineid", "nolabel"},
			want: map[string]string{"machine": "amachineid", "partition": "apartition"},
		},
		{
			name: "gardener specific labels should be ignored",
			tags: []string{"partition=apartition", "machine=amachineid", "networking.gardener.cloud/node-local-dns-enabled=false"},
			want: map[string]string{"machine": "amachineid", "partition": "apartition"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			h := &Housekeeper{}
			got := h.buildLabelsFromMachineTags(tt.tags)
			if !mapsEqual(got, tt.want) {
				t.Errorf("buildLabelsFromMachineTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mapsEqual(map1, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key, value1 := range map1 {
		value2, ok := map2[key]
		if !ok || value1 != value2 {
			return false
		}
	}

	return true
}
