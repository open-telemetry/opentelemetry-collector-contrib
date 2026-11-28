// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadatatest"
)

func TestDefaultConfiguration(t *testing.T) {
	c := createDefaultConfig().(*Config)
	assert.Empty(t, c.AllowedKeys)
	assert.Empty(t, c.BlockedValues)
	assert.Empty(t, c.AllowedValues)
	assert.Empty(t, c.BlockedKeyPatterns)
	assert.Empty(t, c.HashFunction)
}

func TestCreateTestProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.Capabilities().MutatesData)
}

func TestCreateTestLogsProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createLogsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.Capabilities().MutatesData)
}

func TestCreateTestMetricsProcessor(t *testing.T) {
	cfg := &Config{}

	tp, err := createMetricsProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.True(t, tp.Capabilities().MutatesData)
}

func TestCreateProcessorInvalidRegexRecordsErrorMetric(t *testing.T) {
	cfg := &Config{
		BlockedValues: []string{"[invalid"},
	}

	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })

	set := metadatatest.NewSettings(tel)

	_, err := createTracesProcessor(t.Context(), set, cfg, consumertest.NewNop())
	require.Error(t, err)

	metadatatest.AssertEqualProcessorRedactionErrorsTotal(t, tel, []metricdata.DataPoint[int64]{
		{
			Value: 1,
		},
	}, metricdatatest.IgnoreTimestamp())
}
