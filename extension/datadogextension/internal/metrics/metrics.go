// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/metrics"

import (
	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/tagset"
	"go.opentelemetry.io/collector/component"
)

// TagsFromBuildInfo returns a list of tags derived from buildInfo to be used when creating metrics.
func TagsFromBuildInfo(buildInfo component.BuildInfo) []string {
	var tags []string
	if buildInfo.Version != "" {
		tags = append(tags, "version:"+buildInfo.Version)
	}
	if buildInfo.Command != "" {
		tags = append(tags, "command:"+buildInfo.Command)
	}
	return tags
}

// CreateLivenessSerie creates a liveness metric serie to report that the extension is running.
// The timestamp should be in Unix nanoseconds.
func CreateLivenessSerie(hostname string, timestampNs uint64, tags []string) *metrics.Serie {
	// Transform UnixNano timestamp into Unix timestamp (seconds)
	timestamp := float64(timestampNs / 1e9)

	return &metrics.Serie{
		Name:           "otel.datadog_extension.running",
		Points:         []metrics.Point{{Ts: timestamp, Value: 1.0}},
		Tags:           tagset.NewCompositeTags(tags, nil),
		Host:           hostname,
		MType:          metrics.APIGaugeType,
		SourceTypeName: "otel.datadog_extension",
		Source:         metrics.MetricSourceOpenTelemetryCollectorUnknown,
	}
}
