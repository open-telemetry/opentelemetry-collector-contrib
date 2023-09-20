package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestConfig_HTTPPath verifies that the HTTPPath configuration defaults are correctly set for each sub-command.
func TestConfig_HTTPPath(t *testing.T) {
	t.Run("LogsConfigEmptyDefaultUrlPath", func(t *testing.T) {
		assert.Equal(t, "", logsCfg.HTTPPath)
	})

	t.Run("MetricsConfigValidDefaultUrlPath", func(t *testing.T) {
		assert.Equal(t, "/v1/metrics", metricsCfg.HTTPPath)
	})

	t.Run("TracesConfigValidDefaultUrlPath", func(t *testing.T) {
		assert.Equal(t, "/v1/traces", tracesCfg.HTTPPath)
	})
}
