// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awskinesisexporter

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func MustTestGeneric[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

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
					assert.Equal(tb, conf.Region, "us-west-2", "Must match the expected region")
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
			}),
			validateNew: func(tb testing.TB) func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
				return func(conf aws.Config, opts ...func(*kinesis.Options)) *kinesis.Client {
					assert.Equal(tb, conf.Region, "us-east-1", "Must match the expected region")
					k := kinesis.NewFromConfig(conf, opts...)
					return k
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exp, err := createExporter(context.Background(), tc.conf, zaptest.NewLogger(t), func(opt *options) {
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
