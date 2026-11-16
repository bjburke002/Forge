package gitops

import "testing"

func TestParseImage(t *testing.T) {
	tests := []struct {
		in                     string
		name, tag, placeholder string
		wantErr                bool
	}{
		{in: "123456789012.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api:a5a154b",
			name: "123456789012.dkr.ecr.us-east-1.amazonaws.com/forge/payments-api", tag: "a5a154b", placeholder: "payments-api"},
		{in: "localhost:5000/app:v1", name: "localhost:5000/app", tag: "v1", placeholder: "app"}, // registry port is not a tag
		{in: "app:v1", name: "app", tag: "v1", placeholder: "app"},
		{in: "repo/app", wantErr: true},            // no tag
		{in: "localhost:5000/app", wantErr: true},  // port only
		{in: "repo/app:", wantErr: true},           // empty tag
		{in: "repo/app@sha256:abc", wantErr: true}, // digests rejected
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseImage(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := imageRef{name: tc.name, tag: tc.tag, placeholder: tc.placeholder}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}
