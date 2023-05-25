// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestNewMetricsExporter(t *testing.T) {
	got, err := newMetricsExporter(exportertest.NewNopCreateSettings(), &Config{
		Endpoint: "us-west-1.log.aliyuncs.com",
		Project:  "demo-project",
		Logstore: "demo-logstore",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	// This will put trace data to send buffer and return success.
	err = got.ConsumeMetrics(context.Background(), testdata.GenerateMetricsOneMetric())
	assert.NoError(t, err)
}

func TestNewFailsWithEmptyMetricsExporterName(t *testing.T) {
	got, err := newMetricsExporter(exportertest.NewNopCreateSettings(), &Config{})
	assert.Error(t, err)
	require.Nil(t, got)
}
