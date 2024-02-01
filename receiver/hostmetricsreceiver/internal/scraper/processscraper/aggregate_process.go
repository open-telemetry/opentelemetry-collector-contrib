// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"

type aggregateProcessMetadata struct {
	countByStatus    map[metadata.AttributeStatus]int64
	processesCreated int64
}

// Check if any aggregate process metrics are enabled. If neither are enabled,
// we can save on some syscalls by skipping out on calculating them.
func (s *scraper) collectAggregateProcessMetrics() bool {
	return s.config.MetricsBuilderConfig.Metrics.SystemProcessesCount.Enabled ||
		s.config.MetricsBuilderConfig.Metrics.SystemProcessesCreated.Enabled
}
