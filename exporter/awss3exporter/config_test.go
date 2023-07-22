// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.uber.org/multierr"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters["awss3"] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "default.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[component.NewID("awss3")].(*Config)
	assert.Equal(t, e,
		&Config{
			S3Uploader: S3UploaderConfig{
				Region:      "us-east-1",
				S3Bucket:    "foo",
				S3Partition: "minute",
			},
			MarshalerName: "otlp_json",
		},
	)
}

func TestConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[component.NewID("awss3")].(*Config)

	assert.Equal(t, e,
		&Config{
			S3Uploader: S3UploaderConfig{
				Region:      "us-east-1",
				S3Bucket:    "foo",
				S3Prefix:    "bar",
				S3Partition: "minute",
				Endpoint:    "http://endpoint.com",
			},
			MarshalerName: "otlp_json",
		},
	)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		errExpected error
	}{
		{
			name: "valid",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Region = "foo"
				c.S3Uploader.S3Bucket = "bar"
				c.S3Uploader.Endpoint = "http://example.com"
				return c
			}(),
			errExpected: nil,
		},
		{
			name: "missing all",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Region = ""
				return c
			}(),
			errExpected: multierr.Append(errors.New("region is required"),
				errors.New("bucket is required")),
		},
		{
			name: "endpoint and region",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Endpoint = "http://example.com"
				c.S3Uploader.Region = "foo"
				return c
			}(),
			errExpected: errors.New("bucket is required"),
		},
		{
			name: "endpoint and bucket",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Endpoint = "http://example.com"
				c.S3Uploader.S3Bucket = "foo"
				c.S3Uploader.Region = ""
				return c
			}(),
			errExpected: errors.New("region is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Equal(t, tt.errExpected, err)
		})
	}
}
