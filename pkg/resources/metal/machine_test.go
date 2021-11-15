package metal

import "testing"

func Test_decodeMachineIDFromProviderID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		want       string
		wantErr    bool
	}{
		{
			name:       "old format",
			providerID: "metal://amachineid",
			want:       "amachineid",
			wantErr:    false,
		},
		{
			name:       "new format",
			providerID: "metal://apartition/amachineid",
			want:       "amachineid",
			wantErr:    false,
		},
		{
			name:       "new format",
			providerID: "metal://apartition/withslashes/amachineid",
			want:       "amachineid",
			wantErr:    false,
		},
		{
			name:       "wrong provider",
			providerID: "aws://apartition/amachineid",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "wrong format",
			providerID: "aws:/apartition/amachineid",
			want:       "",
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeMachineIDFromProviderID(tt.providerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeMachineIDFromProviderID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("decodeMachineIDFromProviderID() = %v, want %v", got, tt.want)
			}
		})
	}
}
