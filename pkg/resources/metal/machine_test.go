package metal

import "testing"

func Test_decodeMachineIDFromProviderID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		want       string
		wantErr    bool
		err        string
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
			err:        "unexpected providerID format \"aws://apartition/amachineid\", format should be \"metal://<machine-id>\"",
		},
		{
			name:       "wrong format",
			providerID: "metal:/apartition/amachineid",
			want:       "",
			wantErr:    true,
			err:        "unexpected providerID format \"metal:/apartition/amachineid\", format should be \"metal://<machine-id>\"",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeMachineIDFromProviderID(tt.providerID)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeMachineIDFromProviderID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (err != nil) && tt.wantErr && (err.Error() != tt.err) {
				t.Errorf("decodeMachineIDFromProviderID() error = %v, wantErr %v", err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("decodeMachineIDFromProviderID() = %v, want %v", got, tt.want)
			}
		})
	}
}
