// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bufio"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

// only one validate check so far
func TestValidateConfig(t *testing.T) {
	t.Parallel()

	var errs error
	errs = multierr.Append(errs, errMissingEndpointFromConfig)
	errs = multierr.Append(errs, errReadTimeoutExceedsMaxValue)
	errs = multierr.Append(errs, errWriteTimeoutExceedsMaxValue)
	errs = multierr.Append(errs, errRequiredHeader)

	missingEndpointServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	missingEndpointServerConfig.WriteTimeout = 0
	missingEndpointServerConfig.ReadHeaderTimeout = 0
	missingEndpointServerConfig.IdleTimeout = 0
	missingEndpointServerConfig.KeepAlivesEnabled = false
	missingEndpointServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	readTimeoutServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	readTimeoutServerConfig.WriteTimeout = 0
	readTimeoutServerConfig.ReadHeaderTimeout = 0
	readTimeoutServerConfig.IdleTimeout = 0
	readTimeoutServerConfig.KeepAlivesEnabled = false
	readTimeoutServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}

	writeTimeoutServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	writeTimeoutServerConfig.WriteTimeout = 0
	writeTimeoutServerConfig.ReadHeaderTimeout = 0
	writeTimeoutServerConfig.IdleTimeout = 0
	writeTimeoutServerConfig.KeepAlivesEnabled = false
	writeTimeoutServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}

	requiredHeaderKeyServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	requiredHeaderKeyServerConfig.WriteTimeout = 0
	requiredHeaderKeyServerConfig.ReadHeaderTimeout = 0
	requiredHeaderKeyServerConfig.IdleTimeout = 0
	requiredHeaderKeyServerConfig.KeepAlivesEnabled = false
	requiredHeaderKeyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	requiredHeaderValueServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	requiredHeaderValueServerConfig.WriteTimeout = 0
	requiredHeaderValueServerConfig.ReadHeaderTimeout = 0
	requiredHeaderValueServerConfig.IdleTimeout = 0
	requiredHeaderValueServerConfig.KeepAlivesEnabled = false
	requiredHeaderValueServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	multipleInvalidServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	multipleInvalidServerConfig.WriteTimeout = 0
	multipleInvalidServerConfig.ReadHeaderTimeout = 0
	multipleInvalidServerConfig.IdleTimeout = 0
	multipleInvalidServerConfig.KeepAlivesEnabled = false
	multipleInvalidServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		{
			desc:   "Missing valid endpoint",
			expect: errMissingEndpointFromConfig,
			conf: Config{
				ServerConfig: missingEndpointServerConfig,
			},
		},
		{
			desc:   "ReadTimeout exceeds maximum value",
			expect: errReadTimeoutExceedsMaxValue,
			conf: Config{
				ServerConfig: readTimeoutServerConfig,
				ReadTimeout:  "14s",
			},
		},
		{
			desc:   "WriteTimeout exceeds maximum value",
			expect: errWriteTimeoutExceedsMaxValue,
			conf: Config{
				ServerConfig: writeTimeoutServerConfig,
				WriteTimeout: "14s",
			},
		},
		{
			desc:   "RequiredHeader does not contain both a key and a value",
			expect: errRequiredHeader,
			conf: Config{
				ServerConfig: requiredHeaderKeyServerConfig,
				RequiredHeader: RequiredHeader{
					Key:   "key-present",
					Value: "",
				},
			},
		},
		{
			desc:   "RequiredHeader does not contain both a key and a value",
			expect: errRequiredHeader,
			conf: Config{
				ServerConfig: requiredHeaderValueServerConfig,
				RequiredHeader: RequiredHeader{
					Key:   "",
					Value: "value-present",
				},
			},
		},
		{
			desc:   "HMAC missing secret",
			expect: errHMACMissingSecret,
			conf: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
						Endpoint:  "localhost:0",
					},
				},
				HMACSignature: HMACSignature{
					Header: "X-Hub-Signature-256",
					Prefix: "sha256=",
				},
			},
		},
		{
			desc:   "HMAC missing header",
			expect: errHMACMissingHeader,
			conf: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
						Endpoint:  "localhost:0",
					},
				},
				HMACSignature: HMACSignature{
					Secret: "mysecret",
					Prefix: "sha256=",
				},
			},
		},
		{
			desc:   "HMAC missing prefix",
			expect: errHMACMissingPrefix,
			conf: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
						Endpoint:  "localhost:0",
					},
				},
				HMACSignature: HMACSignature{
					Secret: "mysecret",
					Header: "X-Hub-Signature-256",
				},
			},
		},
		{
			desc:   "HMAC valid config",
			expect: nil,
			conf: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
						Endpoint:  "localhost:0",
					},
				},
				HMACSignature: HMACSignature{
					Secret: "mysecret",
					Header: "X-Hub-Signature-256",
					Prefix: "sha256=",
				},
			},
		},
		{
			desc:   "Multiple invalid configs",
			expect: errs,
			conf: Config{
				ServerConfig: multipleInvalidServerConfig,
				WriteTimeout: "14s",
				ReadTimeout:  "15s",
				RequiredHeader: RequiredHeader{
					Key:   "",
					Value: "value-present",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.conf.Validate()
			if test.expect != nil {
				require.ErrorContains(t, err, test.expect.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMaxRequestBodySizeAutoCorrection(t *testing.T) {
	t.Parallel()

	zeroBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	zeroBodySizeServerConfig.WriteTimeout = 0
	zeroBodySizeServerConfig.ReadHeaderTimeout = 0
	zeroBodySizeServerConfig.IdleTimeout = 0
	zeroBodySizeServerConfig.KeepAlivesEnabled = false
	zeroBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	zeroBodySizeServerConfig.MaxRequestBodySize = 0

	smallBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	smallBodySizeServerConfig.WriteTimeout = 0
	smallBodySizeServerConfig.ReadHeaderTimeout = 0
	smallBodySizeServerConfig.IdleTimeout = 0
	smallBodySizeServerConfig.KeepAlivesEnabled = false
	smallBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	smallBodySizeServerConfig.MaxRequestBodySize = 10

	exact64KBBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	exact64KBBodySizeServerConfig.WriteTimeout = 0
	exact64KBBodySizeServerConfig.ReadHeaderTimeout = 0
	exact64KBBodySizeServerConfig.IdleTimeout = 0
	exact64KBBodySizeServerConfig.KeepAlivesEnabled = false
	exact64KBBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	exact64KBBodySizeServerConfig.MaxRequestBodySize = int64(bufio.MaxScanTokenSize)

	greaterBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	greaterBodySizeServerConfig.WriteTimeout = 0
	greaterBodySizeServerConfig.ReadHeaderTimeout = 0
	greaterBodySizeServerConfig.IdleTimeout = 0
	greaterBodySizeServerConfig.KeepAlivesEnabled = false
	greaterBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	greaterBodySizeServerConfig.MaxRequestBodySize = 65538

	wayGreaterBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	wayGreaterBodySizeServerConfig.WriteTimeout = 0
	wayGreaterBodySizeServerConfig.ReadHeaderTimeout = 0
	wayGreaterBodySizeServerConfig.IdleTimeout = 0
	wayGreaterBodySizeServerConfig.KeepAlivesEnabled = false
	wayGreaterBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	wayGreaterBodySizeServerConfig.MaxRequestBodySize = 100 * 1024 * 1024 // 100MB

	tests := []struct {
		desc     string
		conf     Config
		expected int64
	}{
		{
			desc: "MaxRequestBodySize is 0, should be set to default 20MB",
			conf: Config{
				ServerConfig: zeroBodySizeServerConfig,
			},
			expected: 20 * 1024 * 1024, // 20MB default from confighttp
		},
		{
			desc: "MaxRequestBodySize is set to small value, should remain unchanged",
			conf: Config{
				ServerConfig: smallBodySizeServerConfig,
			},
			expected: 10, // No minimum enforcement, user's value is preserved
		},
		{
			desc: "MaxRequestBodySize is exactly 64KB, should remain unchanged",
			conf: Config{
				ServerConfig: exact64KBBodySizeServerConfig,
			},
			expected: int64(bufio.MaxScanTokenSize),
		},
		{
			desc: "MaxRequestBodySize is greater than 64KB, should remain unchanged",
			conf: Config{
				ServerConfig: greaterBodySizeServerConfig,
			},
			expected: 65538,
		},
		{
			desc: "MaxRequestBodySize is way greater than 64KB, should remain unchanged",
			conf: Config{
				ServerConfig: wayGreaterBodySizeServerConfig,
			},
			expected: 100 * 1024 * 1024, // 100MB
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.conf.Validate()
			require.NoError(t, err)
			require.Equal(t, test.expected, test.conf.MaxRequestBodySize)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Skip("skip temporarily to avoid a test failure on read_timeout with https://github.com/open-telemetry/opentelemetry-collector/pull/10275")
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	// LoadConf includes the TypeStr which NewFactory does not set
	id := component.NewIDWithName(metadata.Type, "valid_config")
	cmNoStr, err := cm.Sub(id.String())
	require.NoError(t, err)

	expectServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	expectServerConfig.WriteTimeout = 0
	expectServerConfig.ReadHeaderTimeout = 0
	expectServerConfig.IdleTimeout = 0
	expectServerConfig.KeepAlivesEnabled = false
	expectServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}
	expect := &Config{
		ServerConfig: expectServerConfig,
		ReadTimeout:  "500ms",
		WriteTimeout: "500ms",
		Path:         "some/path",
		HealthPath:   "health/path",
		RequiredHeader: RequiredHeader{
			Key:   "key-present",
			Value: "value-present",
		},
	}

	// create expected config
	factory := NewFactory()
	conf := factory.CreateDefaultConfig()
	require.NoError(t, cmNoStr.Unmarshal(conf))
	require.NoError(t, xconfmap.Validate(conf))

	require.Equal(t, expect, conf)
}
