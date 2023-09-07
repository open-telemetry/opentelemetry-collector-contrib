// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	lokiExporterFailedToSendLogRecordsDueToMissingLabels = stats.Int64("lokiexporter_send_failed_due_to_missing_labels", "Number of log records failed to send because labels were missing", stats.UnitDimensionless)
)

func MetricViews() []*view.View {
	return []*view.View{
		{
			Name:        "lokiexporter_send_failed_due_to_missing_labels",
			Description: "Number of log records failed to send because labels were missing",
			Measure:     lokiExporterFailedToSendLogRecordsDueToMissingLabels,
			Aggregation: view.Count(),
		},
	}
}
