// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !windows

package processscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func TestValidatePlatformEnabledMetrics_Others_DisablesContextSwitches(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessContextSwitches.Enabled = true

	validatePlatformEnabledMetrics(cfg, logger)

	assert.False(t, cfg.Metrics.ProcessContextSwitches.Enabled, "process.context_switches should be disabled")
	assert.Equal(t, 1, logs.Len(), "expected one warning log entry")
	assert.Contains(t, logs.All()[0].Message, "process.context_switches")
}

func TestValidatePlatformEnabledMetrics_Others_DisablesPagingFaults(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessPagingFaults.Enabled = true

	validatePlatformEnabledMetrics(cfg, logger)

	assert.False(t, cfg.Metrics.ProcessPagingFaults.Enabled, "process.paging.faults should be disabled")
	assert.Equal(t, 1, logs.Len(), "expected one warning log entry")
	assert.Contains(t, logs.All()[0].Message, "process.paging.faults")
}

func TestValidatePlatformEnabledMetrics_Others_DisablesSignalsPending(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessSignalsPending.Enabled = true

	validatePlatformEnabledMetrics(cfg, logger)

	assert.False(t, cfg.Metrics.ProcessSignalsPending.Enabled, "process.signals_pending should be disabled")
	assert.Equal(t, 1, logs.Len(), "expected one warning log entry")
	assert.Contains(t, logs.All()[0].Message, "process.signals_pending")
}

func TestValidatePlatformEnabledMetrics_Others_DisablesHandles(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.ProcessHandles.Enabled = true

	validatePlatformEnabledMetrics(cfg, logger)

	assert.False(t, cfg.Metrics.ProcessHandles.Enabled, "process.handles should be disabled")
	assert.Equal(t, 1, logs.Len(), "expected one warning log entry")
	assert.Contains(t, logs.All()[0].Message, "process.handles")
}

func TestValidatePlatformEnabledMetrics_Others_NoopWhenDisabled(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}

	validatePlatformEnabledMetrics(cfg, logger)

	assert.Equal(t, 0, logs.Len(), "no log when metrics are not enabled")
}
