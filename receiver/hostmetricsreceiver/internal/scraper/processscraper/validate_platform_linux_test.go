// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package processscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func TestValidatePlatformEnabledMetrics_Linux_LeavesContextSwitchesEnabled(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessContextSwitches.Enabled = true

	validatePlatformEnabledMetrics(cfg, zap.NewNop())

	assert.True(t, cfg.Metrics.ProcessContextSwitches.Enabled, "process.context_switches should remain enabled on Linux")
}

func TestValidatePlatformEnabledMetrics_Linux_DisablesHandles(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessHandles.Enabled = true

	validatePlatformEnabledMetrics(cfg, logger)

	assert.False(t, cfg.Metrics.ProcessHandles.Enabled, "process.handles should be disabled on Linux")
	assert.Equal(t, 1, logs.Len(), "expected one warning log entry")
	assert.Contains(t, logs.All()[0].Message, "process.handles")
}
