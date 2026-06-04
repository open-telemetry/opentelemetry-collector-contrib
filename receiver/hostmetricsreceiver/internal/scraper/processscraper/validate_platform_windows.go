// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"go.uber.org/zap"
)

// validatePlatformEnabledMetrics disables metrics that are not supported on this platform
// and logs a warning once so the user can correct their config.
//
// Hand-written fallback pending mdatagen `supported_os` annotation support.
// See https://github.com/open-telemetry/opentelemetry-collector/issues/15020;
// replace this with the generated equivalent once it lands.
func validatePlatformEnabledMetrics(cfg *Config, logger *zap.Logger) {
	if cfg.Metrics.ProcessContextSwitches.Enabled {
		logger.Warn("process.context_switches is only supported on Linux; disabling metric on this platform")
		cfg.Metrics.ProcessContextSwitches.Enabled = false
	}
	if cfg.Metrics.ProcessPagingFaults.Enabled {
		logger.Warn("process.paging.faults is only supported on Linux; disabling metric on this platform")
		cfg.Metrics.ProcessPagingFaults.Enabled = false
	}
	if cfg.Metrics.ProcessSignalsPending.Enabled {
		logger.Warn("process.signals_pending is only supported on Linux; disabling metric on this platform")
		cfg.Metrics.ProcessSignalsPending.Enabled = false
	}
}
