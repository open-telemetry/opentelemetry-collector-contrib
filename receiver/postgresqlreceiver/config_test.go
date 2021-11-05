package postgresqlreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc     string
		cfg      *Config
		expected error
	}{
		{
			desc: "missing username and password",
			cfg: &Config{
				SSLConfig: SSLConfig{
					SSLMode: "require",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing password",
			cfg: &Config{
				Username: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "require",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing username",
			cfg: &Config{
				Password: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "require",
				},
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
			),
		},
		{
			desc: "bad SSL mode",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "assume",
				},
			},
			expected: multierr.Combine(
				errors.New("SSL Mode 'assume' not supported, valid values are 'require', 'verify-ca', 'verify-full', 'disable'. The default is 'require'"),
			),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				SSLConfig: SSLConfig{
					SSLMode: "require",
				},
			},
			expected: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			actual := tC.cfg.Validate()
			require.Equal(t, tC.expected, actual)
		})
	}
}
