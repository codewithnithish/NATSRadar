package conn

import (
	"testing"
)

func TestDetermineAuthType(t *testing.T) {
	tests := []struct {
		name     string
		profile  Profile
		expected string
	}{
		{
			name:     "anonymous",
			profile:  Profile{URL: "nats://127.0.0.1:4222"},
			expected: "none",
		},
		{
			name:     "user and password fields",
			profile:  Profile{URL: "nats://127.0.0.1:4222", User: "admin", Password: "secret"},
			expected: "user_password",
		},
		{
			name:     "token field",
			profile:  Profile{URL: "nats://127.0.0.1:4222", Token: "s3cr3t-t0k3n"},
			expected: "token",
		},
		{
			name:     "nkey seed",
			profile:  Profile{URL: "nats://127.0.0.1:4222", NKey: "SUA..."},
			expected: "nkey",
		},
		{
			name:     "credentials content",
			profile:  Profile{URL: "nats://127.0.0.1:4222", Credentials: "-----BEGIN NATS USER JWT-----"},
			expected: "credentials",
		},
		{
			name:     "mtls client certificates",
			profile:  Profile{URL: "tls://127.0.0.1:4443", ClientCert: "CERT", ClientKey: "KEY"},
			expected: "tls_cert",
		},
		{
			name:     "embedded user:pass in url",
			profile:  Profile{URL: "nats://myuser:mypass@127.0.0.1:4222"},
			expected: "user_password",
		},
		{
			name:     "embedded token in url",
			profile:  Profile{URL: "nats://token123@127.0.0.1:4222"},
			expected: "token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := determineAuthType(tc.profile)
			if got != tc.expected {
				t.Errorf("determineAuthType() = %q, expected %q", got, tc.expected)
			}
		})
	}
}
