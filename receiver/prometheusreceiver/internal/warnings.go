// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"slices"

	promconfig "github.com/prometheus/prometheus/config"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

// WarnOpenMetrics2GlobalConfig emits a warning if global.scrape_protocols includes
// OpenMetricsText2.0.0 while the feature gate is disabled, and reports whether
// global.scrape_protocols includes OpenMetricsText2.0.0.
func WarnOpenMetrics2GlobalConfig(logger *zap.Logger, globalProtocols []promconfig.ScrapeProtocol) bool {
	globalHasOM2 := slices.Contains(globalProtocols, promconfig.OpenMetricsText2_0_0)
	if globalHasOM2 && !metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate.IsEnabled() {
		logger.Warn(
			"OpenMetricsText2.0.0 is configured in global.scrape_protocols, but the feature gate is disabled; OpenMetrics 2.0 responses will fall back to fallback_scrape_protocol (or fail if none is configured)",
			zap.String("feature_gate", metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate.ID()),
		)
	}
	return globalHasOM2
}

// WarnOpenMetrics2ScrapeConfig emits warnings for OpenMetrics 2.0 misconfigurations on a scrape job.
func WarnOpenMetrics2ScrapeConfig(logger *zap.Logger, sc *promconfig.ScrapeConfig, globalHasOM2 bool, globalProtocols []promconfig.ScrapeProtocol) {
	if sc == nil {
		return
	}
	if !metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate.IsEnabled() &&
		slices.Contains(sc.ScrapeProtocols, promconfig.OpenMetricsText2_0_0) &&
		(!globalHasOM2 || !slices.Equal(sc.ScrapeProtocols, globalProtocols)) {
		logger.Warn(
			"OpenMetricsText2.0.0 is configured in scrape_protocols, but the feature gate is disabled; OpenMetrics 2.0 responses will fall back to fallback_scrape_protocol (or fail if none is configured)",
			zap.String("job", sc.JobName),
			zap.String("feature_gate", metadata.ReceiverPrometheusEnableOpenMetrics2FeatureGate.ID()),
		)
	}
	if sc.ScrapeFallbackProtocol == promconfig.OpenMetricsText2_0_0 {
		logger.Warn(
			"fallback_scrape_protocol OpenMetricsText2.0.0 is not supported by the pinned Prometheus scrape library and currently falls back to OpenMetrics 1.0",
			zap.String("job", sc.JobName),
		)
	}
}
