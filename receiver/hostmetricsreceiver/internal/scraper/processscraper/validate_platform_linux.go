// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

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
	if cfg.Metrics.ProcessHandles.Enabled {
		logger.Warn("process.handles is only supported on Windows; disabling metric on this platform")
		cfg.Metrics.ProcessHandles.Enabled = false
	}
}
