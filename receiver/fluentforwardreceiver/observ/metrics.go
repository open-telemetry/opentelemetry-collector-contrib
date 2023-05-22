// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package observ contains logic pertaining to the internal observation
// of the fluent forward receiver.
package observ // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/observ"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// ConnectionsOpened measure for number of connections opened to the fluentforward receiver.
	ConnectionsOpened = stats.Int64(
		"fluent_opened_connections",
		"Number of connections opened to the fluentforward receiver",
		stats.UnitDimensionless)
	connectionsOpenedView = &view.View{
		Name:        ConnectionsOpened.Name(),
		Measure:     ConnectionsOpened,
		Description: ConnectionsOpened.Description(),
		Aggregation: view.Sum(),
	}

	// ConnectionsClosed measure for number of connections closed to the fluentforward receiver.
	ConnectionsClosed = stats.Int64(
		"fluent_closed_connections",
		"Number of connections closed to the fluentforward receiver",
		stats.UnitDimensionless)
	connectionsClosedView = &view.View{
		Name:        ConnectionsClosed.Name(),
		Measure:     ConnectionsClosed,
		Description: ConnectionsClosed.Description(),
		Aggregation: view.Sum(),
	}

	// EventsParsed measure for number of Fluent events parsed successfully.
	EventsParsed = stats.Int64(
		"fluent_events_parsed",
		"Number of Fluent events parsed successfully",
		stats.UnitDimensionless)
	eventsParsedView = &view.View{
		Name:        EventsParsed.Name(),
		Measure:     EventsParsed,
		Description: EventsParsed.Description(),
		Aggregation: view.Sum(),
	}

	// FailedToParse measure for number of times Fluent messages failed to be decoded.
	FailedToParse = stats.Int64(
		"fluent_parse_failures",
		"Number of times Fluent messages failed to be decoded",
		stats.UnitDimensionless)
	failedToParseView = &view.View{
		Name:        FailedToParse.Name(),
		Measure:     FailedToParse,
		Description: FailedToParse.Description(),
		Aggregation: view.Sum(),
	}

	// RecordsGenerated measure for number of log records generated from Fluent forward input.
	RecordsGenerated = stats.Int64(
		"fluent_records_generated",
		"Number of log records generated from Fluent forward input",
		stats.UnitDimensionless)
	recordsGeneratedView = &view.View{
		Name:        RecordsGenerated.Name(),
		Measure:     RecordsGenerated,
		Description: RecordsGenerated.Description(),
		Aggregation: view.Sum(),
	}
)

func MetricViews() []*view.View {
	return []*view.View{
		connectionsOpenedView,
		connectionsClosedView,
		eventsParsedView,
		failedToParseView,
		recordsGeneratedView,
	}
}
