// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter/internal/metadata"
)

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Bucket.Region = "test-region"
	cfg.Bucket.ProjectID = "test-project"
	cfg.Bucket.Name = "test-bucket"

	exp, err := createMetricsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Bucket.Region = "test-region"
	cfg.Bucket.ProjectID = "test-project"
	cfg.Bucket.Name = "test-bucket"

	exp, err := createLogsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Bucket.Region = "test-region"
	cfg.Bucket.ProjectID = "test-project"
	cfg.Bucket.Name = "test-bucket"

	exp, err := createTracesExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, exp)
}
