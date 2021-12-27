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
		endpoints []string
		desc      string
		username  string
		password  string
		expected  error
	}{
		{
			desc:      "no username, no password",
			endpoints: []string{"localhost:27107"},
			username:  "",
			password:  "",
			expected:  nil,
		},
		{
			desc:      "no username, with password",
			endpoints: []string{"localhost:27107"},
			username:  "",
			password:  "pass",
			expected:  errors.New("password provided without user"),
		},
		{
			desc:      "with username, no password",
			endpoints: []string{"localhost:27107"},
			username:  "user",
			password:  "",
			expected:  errors.New("username provided without password"),
		},
		{
			desc:      "with username and password",
			endpoints: []string{"localhost:27107"},
			username:  "user",
			password:  "pass",
			expected:  nil,
		},
		{
			desc:     "no hosts",
			username: "user",
			password: "pass",
			expected: errors.New("no hosts were specified in the config"),
		},
		{
			desc:      "empty host",
			username:  "user",
			endpoints: []string{""},
			expected:  errors.New("does not match format of '<host>:<port>"),
		},
		{
			desc:      "bad endpoint format",
			endpoints: []string{"localhost;27017]"},
			username:  "user",
			password:  "pass",
			expected:  errors.New("does not match format of '<host>:<port>'"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hosts := []confignet.TCPAddr{}

			for _, ep := range tc.endpoints {
				hosts = append(hosts, confignet.TCPAddr{
					Endpoint: ep,
				})
			}

			cfg := Config{
				Username: tc.username,
				Password: tc.password,
				Hosts:    hosts,
			}
			err := cfg.Validate()
			if tc.expected == nil {
				require.Nil(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expected.Error())
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
			cfg := Config{
				Username: "otel",
				Password: "pword",
				Hosts: []confignet.TCPAddr{
					{
						Endpoint: "localhost:27017",
					},
				},
				TLSClientSetting: tc.tlsConfig,
			}
			err := cfg.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
