// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudmysqlduckdbexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, metadata.Type, factory.Type())

	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	c := cfg.(*Config)
	assert.Equal(t, "otel", c.Database)
	assert.Equal(t, "otel_logs", c.LogsTableName)
	assert.Equal(t, "otel_traces", c.TracesTableName)
	assert.Equal(t, "otel_metrics", c.MetricsTableName)
	assert.True(t, c.CreateSchema)
}
