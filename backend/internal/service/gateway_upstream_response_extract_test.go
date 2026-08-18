package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractUpstreamErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "claude object error",
			body: `{"type":"error","error":{"type":"invalid_request_error","message":"bad temperature"}}`,
			want: "bad temperature",
		},
		{
			name: "nested json string in message",
			body: `{"error":{"message":"{\"error\":{\"message\":\"inner failure\"}}"}}`,
			want: "inner failure",
		},
		{
			name: "xai string error",
			body: `{"error":"Failed to deserialize the JSON body into the target type: data did not match any variant of untagged enum ModelInput"}`,
			want: "Failed to deserialize the JSON body into the target type: data did not match any variant of untagged enum ModelInput",
		},
		{
			name: "detail style",
			body: `{"detail":"token expired"}`,
			want: "token expired",
		},
		{
			name: "top-level message fallback",
			body: `{"message":"plain failure"}`,
			want: "plain failure",
		},
		{
			name: "object error without message does not leak raw json",
			body: `{"error":{"code":"bad_request"}}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, extractUpstreamErrorMessage([]byte(tt.body)))
		})
	}
}
