// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awsecscontainermetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func TestConvertToOTMetrics(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100

	resource := pcommon.NewResource()
	md := convertToOTLPMetrics("container.", m, resource, timestamp)
	require.Equal(t, 26, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	assert.Equal(t, conventions.SchemaURL, md.ResourceMetrics().At(0).SchemaUrl())
}

func TestIntGauge(t *testing.T) {
	intValue := int64(100)
	timestamp := pcommon.NewTimestampFromTime(time.Now())

	ilm := pmetric.NewScopeMetrics()
	appendIntGauge("cpu_utilized", "Count", intValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestDoubleGauge(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	floatValue := 100.01

	ilm := pmetric.NewScopeMetrics()
	appendDoubleGauge("cpu_utilized", "Count", floatValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestIntSum(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	intValue := int64(100)

	ilm := pmetric.NewScopeMetrics()
	appendIntSum("cpu_utilized", "Count", intValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestConvertStoppedContainerDataToOTMetrics(t *testing.T) {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	resource := pcommon.NewResource()
	duration := 1200000000.32132
	md := convertStoppedContainerDataToOTMetrics("container.", resource, timestamp, duration)
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
}
