// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
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
