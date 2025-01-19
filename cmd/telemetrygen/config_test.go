// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfig_HTTPPath verifies that the HTTPPath configuration defaults are correctly set for each sub-command.
func TestConfig_HTTPPath(t *testing.T) {
	t.Run("LogsConfigValidDefaultUrlPath", func(t *testing.T) {
		assert.Equal(t, "/v1/logs", logsCfg.HTTPPath)
	})

	t.Run("MetricsConfigValidDefaultUrlPath", func(t *testing.T) {
		assert.Equal(t, "/v1/metrics", metricsCfg.HTTPPath)
	})

	t.Run("TracesConfigValidDefaultUrlPath", func(t *testing.T) {
		assert.Equal(t, "/v1/traces", tracesCfg.HTTPPath)
	})
}
