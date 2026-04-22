// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package processscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func TestValidatePlatformEnabledMetrics_Linux_LeavesContextSwitchesEnabled(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessContextSwitches.Enabled = true

	validatePlatformEnabledMetrics(cfg, zap.NewNop())

	assert.True(t, cfg.Metrics.ProcessContextSwitches.Enabled, "process.context_switches should remain enabled on Linux")
}
