// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Copyright © 2025, Oracle and/or its affiliates.

package oracleobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/oracleobservabilityexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/oracleobservabilityexporter/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	t.Parallel()

	factory := NewFactory()

	assert.NotNil(t, factory, "expected a non-nil factory")
	assert.Equal(t, factory.Type().String(), metadata.Type.String(), "expected factory type to be 'oracleobservability'")
}

func TestCreateDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := createDefaultConfig()
	config := cfg.(*Config)
	config.NamespaceName = "test-namespace"
	config.LogGroupID = "test-log-group"

	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateLogsExporter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	params := exportertest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()

	config := cfg.(*Config)
	config.AuthType = "config_file"
	config.NamespaceName = "test-namespace"
	config.LogGroupID = "test-log-group"
	config.ConfigFilePath = "testdata/demo_oci_config_file"
	config.ConfigProfile = "DEFAULT"

	exporter, err := createLogsExporter(ctx, params, config)

	assert.NoError(t, err, "expected no error while creating logs exporter")
	assert.NotNil(t, exporter, "expected a non-nil logs exporter")
	require.NoError(t, exporter.Shutdown(t.Context()))
}
