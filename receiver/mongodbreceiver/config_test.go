package mongodbreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"gotest.tools/assert"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		endpoint string
		desc     string
		username string
		password string
		expected error
	}{
		{
			desc:     "no username, no password",
			endpoint: "localhost:27107",
			username: "",
			password: "",
			expected: nil,
		},
		{
			desc:     "no username, with password",
			endpoint: "localhost:27107",
			username: "",
			password: "pass",
			expected: errors.New("password provided without user"),
		},
		{
			desc:     "with username, no password",
			endpoint: "localhost:27107",
			username: "user",
			password: "",
			expected: errors.New("user provided without password"),
		},
		{
			desc:     "with username and password",
			endpoint: "localhost:27107",
			username: "user",
			password: "pass",
			expected: nil,
		},
		{
			desc:     "no endpoint",
			username: "user",
			password: "pass",
			expected: errors.New("no endpoint specified"),
		},
		{
			desc:     "bad endpoint format",
			endpoint: "localhost;27017]",
			username: "user",
			password: "pass",
			expected: errors.New("endpoint does not match format of 'host:port'"),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			cfg := Config{Username: tC.username, Password: tC.password, TCPAddr: confignet.TCPAddr{
				Endpoint: tC.endpoint,
			}}
			err := cfg.Validate()
			if tC.expected == nil {
				require.Nil(t, err)
			} else {
				assert.ErrorContains(t, err, tC.expected.Error())
			}
		})
	}
}

func TestBadTLSConfigs(t *testing.T) {
	testCases := []struct {
		desc        string
		tlsConfig   configtls.TLSClientSetting
		expectError bool
	}{
		{
			desc: "CA file not found",
			tlsConfig: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "not/a/real/file.pem",
				},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			expectError: true,
		},
		{
			desc: "no issues",
			tlsConfig: configtls.TLSClientSetting{
				TLSSetting:         configtls.TLSSetting{},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := Config{Username: "otel", Password: "pword", TCPAddr: confignet.TCPAddr{Endpoint: "localhost:27017"}, TLSClientSetting: tc.tlsConfig}
			err := cfg.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
