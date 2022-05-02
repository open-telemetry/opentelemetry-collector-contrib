package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "blank endpoint",
			config: &Config{
				Endpoint: "",
			},
			expected: errEmptyEndpoint,
		},
		{
			name: "missing port",
			config: &Config{
				Endpoint: "localhost",
			},
			expected: errBadEndpoint,
		},
		{
			name: "missing host",
			config: &Config{
				Endpoint: ":3001",
			},
			expected: errBadEndpoint,
		},
		{
			name: "negative port",
			config: &Config{
				Endpoint: "localhost:-2",
			},
			expected: errBadPort,
		},
		{
			name: "bad port",
			config: &Config{
				Endpoint: "localhost:2.02",
			},
			expected: errBadPort,
		},
		{
			name: "password but no username",
			config: &Config{
				Endpoint: "localhost:3001",
				Username: "",
				Password: "secret",
			},
			expected: errEmptyUsername,
		},
		{
			name: "username but no password",
			config: &Config{
				Endpoint: "localhost:3001",
				Username: "ro_user",
			},
			expected: errEmptyPassword,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			require.ErrorIs(t, err, tc.expected)
		})
	}
}
