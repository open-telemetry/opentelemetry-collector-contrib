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
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "default.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[component.MustNewID("awss3")].(*Config)
	encoding := component.MustNewIDWithName("foo", "bar")

	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Enabled = false

	assert.Equal(t, &Config{
		QueueSettings:         queueCfg,
		Encoding:              &encoding,
		EncodingFileExtension: "baz",
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "foo",
			S3Partition:  "minute",
			StorageClass: "STANDARD",
		},
		MarshalerName: "otlp_json",
	}, e,
	)
}

func TestConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	queueCfg := exporterhelper.QueueConfig{
		Enabled:      true,
		NumConsumers: 23,
		QueueSize:    42,
	}

	e := cfg.Exporters[component.MustNewID("awss3")].(*Config)

	assert.Equal(t, &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "foo",
			S3Prefix:     "bar",
			S3Partition:  "minute",
			Endpoint:     "http://endpoint.com",
			StorageClass: "STANDARD",
		},
		MarshalerName: "otlp_json",
	}, e,
	)
}

func TestConfigS3StorageClaas(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	cfg, err := otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "config-s3_storage_class.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e := cfg.Exporters[component.MustNewID("awss3")].(*Config)
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Enabled = false

	assert.Equal(t, &Config{
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "foo",
			S3Prefix:     "bar",
			S3Partition:  "minute",
			Endpoint:     "http://endpoint.com",
			StorageClass: "STANDARD_IA",
		},
		QueueSettings: queueCfg,
		MarshalerName: "otlp_json",
	}, e,
	)
}

func TestConfigForS3CompatibleSystems(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "config-s3-compatible-systems.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Enabled = false

	e := cfg.Exporters[component.MustNewID("awss3")].(*Config)

	assert.Equal(t, &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:           "us-east-1",
			S3Bucket:         "foo",
			S3Prefix:         "bar",
			S3Partition:      "minute",
			Endpoint:         "alternative-s3-system.example.com",
			S3ForcePathStyle: true,
			DisableSSL:       true,
			StorageClass:     "STANDARD",
		},
		MarshalerName: "otlp_json",
	}, e,
	)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		errExpected error
	}{
		{
			// endpoint overrides region and bucket name.
			name: "valid with endpoint and region",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Endpoint = "http://example.com"
				c.S3Uploader.Region = "foo"
				return c
			}(),
			errExpected: nil,
		},
		{
			// Endpoint will be built from bucket and region.
			// https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
			name: "valid with S3Bucket and region",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Region = "foo"
				c.S3Uploader.S3Bucket = "bar"
				return c
			}(),
			errExpected: nil,
		},
		{
			name: "missing all",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Region = ""
				c.S3Uploader.S3Bucket = ""
				c.S3Uploader.Endpoint = ""
				return c
			}(),
			errExpected: multierr.Append(errors.New("region is required"),
				errors.New("bucket or endpoint is required")),
		},
		{
			name: "region only",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Region = "foo"
				c.S3Uploader.S3Bucket = ""
				return c
			}(),
			errExpected: errors.New("bucket or endpoint is required"),
		},
		{
			name: "bucket only",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.S3Bucket = "foo"
				c.S3Uploader.Region = ""
				return c
			}(),
			errExpected: errors.New("region is required"),
		},
		{
			name: "endpoint only",
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.S3Uploader.Endpoint = "http://example.com"
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

func TestMarshallerName(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "marshaler.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Enabled = false

	e := cfg.Exporters[component.MustNewID("awss3")].(*Config)

	assert.Equal(t, &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "foo",
			S3Partition:  "minute",
			StorageClass: "STANDARD",
		},
		MarshalerName: "sumo_ic",
	}, e,
	)

	e = cfg.Exporters[component.MustNewIDWithName("awss3", "proto")].(*Config)

	assert.Equal(t, &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "bar",
			S3Partition:  "minute",
			StorageClass: "STANDARD",
		},
		MarshalerName: "otlp_proto",
	}, e,
	)
}

func TestCompressionName(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[factory.Type()] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "compression.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Enabled = false

	e := cfg.Exporters[component.MustNewID("awss3")].(*Config)

	assert.Equal(t, &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "foo",
			S3Partition:  "minute",
			Compression:  "gzip",
			StorageClass: "STANDARD",
		},
		MarshalerName: "otlp_json",
	}, e,
	)

	e = cfg.Exporters[component.MustNewIDWithName("awss3", "proto")].(*Config)

	assert.Equal(t, &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:       "us-east-1",
			S3Bucket:     "bar",
			S3Partition:  "minute",
			Compression:  "none",
			StorageClass: "STANDARD",
		},
		MarshalerName: "otlp_proto",
	}, e,
	)
}
