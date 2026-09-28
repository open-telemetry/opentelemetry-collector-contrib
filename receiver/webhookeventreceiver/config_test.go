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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
	missingEndpointServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	readTimeoutServerConfig := confighttp.NewDefaultServerConfig()
	readTimeoutServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}

	writeTimeoutServerConfig := confighttp.NewDefaultServerConfig()
	writeTimeoutServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}

	requiredHeaderKeyServerConfig := confighttp.NewDefaultServerConfig()
	requiredHeaderKeyServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	requiredHeaderValueServerConfig := confighttp.NewDefaultServerConfig()
	requiredHeaderValueServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "",
	}

	multipleInvalidServerConfig := confighttp.NewDefaultServerConfig()
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
	zeroBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	zeroBodySizeServerConfig.MaxRequestBodySize = 0

	smallBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	smallBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	smallBodySizeServerConfig.MaxRequestBodySize = 10

	exact64KBBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	exact64KBBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	exact64KBBodySizeServerConfig.MaxRequestBodySize = int64(bufio.MaxScanTokenSize)

	greaterBodySizeServerConfig := confighttp.NewDefaultServerConfig()
	greaterBodySizeServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:0",
	}
	greaterBodySizeServerConfig.MaxRequestBodySize = 65538

	wayGreaterBodySizeServerConfig := confighttp.NewDefaultServerConfig()
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
			require.Equal(t, test.expected, test.conf.ServerConfig.MaxRequestBodySize)
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
	require.NoError(t, confmap.Validate(conf))

	require.Equal(t, expect, conf)
}
