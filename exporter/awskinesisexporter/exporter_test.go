// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awskinesisexporter

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func applyConfigChanges(fn func(conf *Config)) *Config {
	conf := createDefaultConfig().(*Config)
	fn(conf)
	return conf
}

func TestCreatingExporter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		conf        *Config
		validateNew func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client
		err         error
	}{
		{
			name: "Default configuration",
			conf: applyConfigChanges(func(conf *Config) {
				conf.AWS.StreamName = "example-test"
			}),
			validateNew: func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
				return func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
					assert.Equal(tb, "us-west-2", conf.Region, "Must match the expected region")
					require.NotNil(tb, conf.HTTPClient, "HTTPClient must not be nil")
					buildableClient, ok := conf.HTTPClient.(*awshttp.BuildableClient)
					require.True(tb, ok, "HTTPClient must be a *awshttp.BuildableClient")
					tr := buildableClient.GetTransport()
					require.NotNil(tb, tr, "Transport must not be nil")
					assert.Equal(tb, 90*time.Second, tr.IdleConnTimeout, "Default idle connection timeout must be 90s")
					assert.Equal(tb, 100, tr.MaxIdleConns)
					assert.Equal(tb, 10, tr.MaxIdleConnsPerHost)
					assert.Equal(tb, 2048, tr.MaxConnsPerHost)
					require.NotNil(tb, tr.TLSClientConfig)
					assert.Equal(tb, uint16(tls.VersionTLS12), tr.TLSClientConfig.MinVersion)

					k := kinesis.NewFromConfig(conf, opts...)
					return k
				}
			},
		},
		{
			name: "Apply different region",
			conf: applyConfigChanges(func(conf *Config) {
				conf.AWS.StreamName = "example-test"
				conf.AWS.Region = "us-east-1"
				conf.AWS.Role = "example-role"
			}),
			validateNew: func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
				return func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
					assert.Equal(tb, "us-east-1", conf.Region, "Must match the expected region")
					k := kinesis.NewFromConfig(conf, opts...)
					return k
				}
			},
		},
		{
			name: "Custom 2-second idle connection timeout",
			conf: applyConfigChanges(func(conf *Config) {
				conf.AWS.StreamName = "example-test"
				conf.IdleConnTimeout = 2 * time.Second
			}),
			validateNew: func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
				return func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
					require.NotNil(tb, conf.HTTPClient, "HTTPClient must not be nil")
					buildableClient, ok := conf.HTTPClient.(*awshttp.BuildableClient)
					require.True(tb, ok, "HTTPClient must be a *awshttp.BuildableClient")
					tr := buildableClient.GetTransport()
					require.NotNil(tb, tr, "Transport must not be nil")
					assert.Equal(tb, 2*time.Second, tr.IdleConnTimeout, "Custom idle connection timeout must be 2s")
					k := kinesis.NewFromConfig(conf, opts...)
					return k
				}
			},
		},
		{
			name: "Custom longer idle connection timeout",
			conf: applyConfigChanges(func(conf *Config) {
				conf.AWS.StreamName = "example-test"
				conf.IdleConnTimeout = 120 * time.Second
			}),
			validateNew: func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
				return func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
					require.NotNil(tb, conf.HTTPClient, "HTTPClient must not be nil")
					buildableClient, ok := conf.HTTPClient.(*awshttp.BuildableClient)
					require.True(tb, ok, "HTTPClient must be a *awshttp.BuildableClient")
					tr := buildableClient.GetTransport()
					require.NotNil(tb, tr, "Transport must not be nil")
					assert.Equal(tb, 120*time.Second, tr.IdleConnTimeout, "Custom idle connection timeout must be 120s")
					k := kinesis.NewFromConfig(conf, opts...)
					return k
				}
			},
		},
		{
			name: "Zero idle connection timeout falls back to 90s default",
			conf: applyConfigChanges(func(conf *Config) {
				conf.AWS.StreamName = "example-test"
				conf.IdleConnTimeout = 0
			}),
			validateNew: func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
				return func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
					require.NotNil(tb, conf.HTTPClient, "HTTPClient must not be nil")
					buildableClient, ok := conf.HTTPClient.(*awshttp.BuildableClient)
					require.True(tb, ok, "HTTPClient must be a *awshttp.BuildableClient")
					tr := buildableClient.GetTransport()
					require.NotNil(tb, tr, "Transport must not be nil")
					assert.Equal(tb, 90*time.Second, tr.IdleConnTimeout, "Zero timeout must fall back to 90s default")
					k := kinesis.NewFromConfig(conf, opts...)
					return k
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exp, err := createExporter(t.Context(), tc.conf, zaptest.NewLogger(t), func(opt *options) {
				opt.NewKinesisClient = tc.validateNew(t)
			})
			assert.ErrorIs(t, err, tc.err, "Must match the expected error")
			if tc.err != nil {
				assert.Nil(t, exp, "Must be nil if error returned")
				return
			}
			assert.NotNil(t, exp, "Must not be nil if no error is returned")
		})
	}
}
