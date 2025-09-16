// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package environment

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal"
)

// TestCollector_Name tests the collector name
func TestCollector_Name(t *testing.T) {
	collector := NewCollector()
	assert.Equal(t, "environment", collector.Name())
}

// TestCollector_GetMetricNames tests metric names match cisco_exporter format
func TestCollector_GetMetricNames(t *testing.T) {
	collector := NewCollector()
	expected := []string{
		internal.MetricPrefix + "environment_sensor_temp",
		internal.MetricPrefix + "environment_power_up",
	}
	assert.Equal(t, expected, collector.GetMetricNames())
}

// TestCollector_GetRequiredCommands tests required commands
func TestCollector_GetRequiredCommands(t *testing.T) {
	collector := NewCollector()
	expected := []string{"show environment"}
	assert.Equal(t, expected, collector.GetRequiredCommands())
}

// TestCollector_Components tests collector components are properly initialized
func TestCollector_Components(t *testing.T) {
	collector := NewCollector()
	assert.NotNil(t, collector.parser, "Parser should be initialized")
	assert.NotNil(t, collector.metricBuilder, "MetricBuilder should be initialized")
}
