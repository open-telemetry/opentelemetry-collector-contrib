// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package observability // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/observability"

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

func init() {
	err := view.Register(
		viewRequestsSent,
		viewRequestsDuration,
		viewRequestsBytes,
		viewRequestsRecords,
	)
	if err != nil {
		fmt.Printf("Failed to register sumologic exporter's views: %v\n", err)
	}
}

var (
	mRequestsSent     = stats.Int64("exporter/requests/sent", "Number of requests", "1")
	mRequestsDuration = stats.Int64("exporter/requests/duration", "Duration of HTTP requests (in milliseconds)", "0")
	mRequestsBytes    = stats.Int64("exporter/requests/bytes", "Total size of requests (in bytes)", "0")
	mRequestsRecords  = stats.Int64("exporter/requests/records", "Total size of requests (in number of records)", "0")

	statusKey, _   = tag.NewKey("status_code") // nolint:errcheck
	endpointKey, _ = tag.NewKey("endpoint")    // nolint:errcheck
	pipelineKey, _ = tag.NewKey("pipeline")    // nolint:errcheck
	exporterKey, _ = tag.NewKey("exporter")    // nolint:errcheck
)

var viewRequestsSent = &view.View{
	Name:        mRequestsSent.Name(),
	Description: mRequestsSent.Description(),
	Measure:     mRequestsSent,
	TagKeys:     []tag.Key{statusKey, endpointKey, pipelineKey, exporterKey},
	Aggregation: view.Count(),
}

var viewRequestsDuration = &view.View{
	Name:        mRequestsDuration.Name(),
	Description: mRequestsDuration.Description(),
	Measure:     mRequestsDuration,
	TagKeys:     []tag.Key{statusKey, endpointKey, pipelineKey, exporterKey},
	Aggregation: view.Sum(),
}

var viewRequestsBytes = &view.View{
	Name:        mRequestsBytes.Name(),
	Description: mRequestsBytes.Description(),
	Measure:     mRequestsBytes,
	TagKeys:     []tag.Key{statusKey, endpointKey, pipelineKey, exporterKey},
	Aggregation: view.Sum(),
}

var viewRequestsRecords = &view.View{
	Name:        mRequestsRecords.Name(),
	Description: mRequestsRecords.Description(),
	Measure:     mRequestsRecords,
	TagKeys:     []tag.Key{statusKey, endpointKey, pipelineKey, exporterKey},
	Aggregation: view.Sum(),
}

// RecordRequestsSent increments the metric that records sent requests
func RecordRequestsSent(statusCode int, endpoint string, pipeline string, exporter string) error {
	return stats.RecordWithTags(
		context.Background(),
		[]tag.Mutator{
			tag.Insert(statusKey, fmt.Sprint(statusCode)),
			tag.Insert(endpointKey, endpoint),
			tag.Insert(pipelineKey, pipeline),
			tag.Insert(exporterKey, exporter),
		},
		mRequestsSent.M(int64(1)),
	)
}

// RecordRequestsDuration update metric which records request duration
func RecordRequestsDuration(duration time.Duration, statusCode int, endpoint string, pipeline string, exporter string) error {
	return stats.RecordWithTags(
		context.Background(),
		[]tag.Mutator{
			tag.Insert(statusKey, fmt.Sprint(statusCode)),
			tag.Insert(endpointKey, endpoint),
			tag.Insert(pipelineKey, pipeline),
			tag.Insert(exporterKey, exporter),
		},
		mRequestsDuration.M(duration.Milliseconds()),
	)
}

// RecordRequestsBytes update metric which records number of send bytes
func RecordRequestsBytes(bytes int64, statusCode int, endpoint string, pipeline string, exporter string) error {
	return stats.RecordWithTags(
		context.Background(),
		[]tag.Mutator{
			tag.Insert(statusKey, fmt.Sprint(statusCode)),
			tag.Insert(endpointKey, endpoint),
			tag.Insert(pipelineKey, pipeline),
			tag.Insert(exporterKey, exporter),
		},
		mRequestsBytes.M(bytes),
	)
}

// RecordRequestsRecords update metric which records number of sent records
func RecordRequestsRecords(records int64, statusCode int, endpoint string, pipeline string, exporter string) error {
	return stats.RecordWithTags(
		context.Background(),
		[]tag.Mutator{
			tag.Insert(statusKey, fmt.Sprint(statusCode)),
			tag.Insert(endpointKey, endpoint),
			tag.Insert(pipelineKey, pipeline),
			tag.Insert(exporterKey, exporter),
		},
		mRequestsRecords.M(records),
	)
}
