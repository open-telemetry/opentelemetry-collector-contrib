// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bgp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollector_Name(t *testing.T) {
	collector := NewCollector()
	assert.Equal(t, "bgp", collector.Name())
}

func TestCollector_Components(t *testing.T) {
	collector := NewCollector()

	// Test that collector has required components
	assert.NotNil(t, collector)
	assert.Equal(t, "bgp", collector.Name())
}

func TestCollector_GetMetricNames(t *testing.T) {
	collector := NewCollector()
	metricNames := collector.GetMetricNames()

	expected := []string{
		"cisco_bgp_session_up",
		"cisco_bgp_session_prefixes_received_count",
		"cisco_bgp_session_messages_input_count",
		"cisco_bgp_session_messages_output_count",
	}

	assert.ElementsMatch(t, expected, metricNames)
}

func TestCollector_GetRequiredCommands(t *testing.T) {
	collector := NewCollector()
	commands := collector.GetRequiredCommands()

	expected := []string{"show bgp all summary"}
	assert.ElementsMatch(t, expected, commands)
}
