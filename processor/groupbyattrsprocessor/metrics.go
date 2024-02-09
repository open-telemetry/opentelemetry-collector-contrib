// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

type internalTelemetry struct {
	mNumGroupedSpans    metric.Int64Counter
	mNumNonGroupedSpans metric.Int64Counter
	mDistSpanGroups     metric.Int64Histogram

	mNumGroupedLogs    metric.Int64Counter
	mNumNonGroupedLogs metric.Int64Counter
	mDistLogGroups     metric.Int64Histogram

	mNumGroupedMetrics    metric.Int64Counter
	mNumNonGroupedMetrics metric.Int64Counter
	mDistMetricGroups     metric.Int64Histogram
}

func newProcessorTelemetry(set component.TelemetrySettings) (*internalTelemetry, error) {
	it := internalTelemetry{}

	counter, err := metadata.Meter(set).Int64Counter(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "num_grouped_spans"),
		metric.WithDescription("Number of spans that had attributes grouped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	it.mNumGroupedSpans = counter

	counter, err = metadata.Meter(set).Int64Counter(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "num_non_grouped_spans"),
		metric.WithDescription("Number of spans that did not have attributes grouped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	it.mNumNonGroupedSpans = counter

	histo, err := metadata.Meter(set).Int64Histogram(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "span_groups"),
		metric.WithDescription("Distribution of groups extracted for spans"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	it.mDistSpanGroups = histo

	counter, err = metadata.Meter(set).Int64Counter(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "num_grouped_logs"),
		metric.WithDescription("Number of logs that had attributes grouped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	it.mNumGroupedLogs = counter

	counter, err = metadata.Meter(set).Int64Counter(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "num_non_grouped_logs"),
		metric.WithDescription("Number of logs that did not have attributes grouped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	it.mNumNonGroupedLogs = counter

	histo, err = metadata.Meter(set).Int64Histogram(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "log_groups"),
		metric.WithDescription("Distribution of groups extracted for logs"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	it.mDistLogGroups = histo

	counter, err = metadata.Meter(set).Int64Counter(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "num_grouped_metrics"),
		metric.WithDescription("Number of metrics that had attributes grouped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	it.mNumGroupedMetrics = counter

	counter, err = metadata.Meter(set).Int64Counter(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "num_non_grouped_metrics"),
		metric.WithDescription("Number of metrics that did not have attributes grouped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	it.mNumNonGroupedMetrics = counter

	histo, err = metadata.Meter(set).Int64Histogram(
		processorhelper.BuildCustomMetricName(metadata.Type.String(), "metric_groups"),
		metric.WithDescription("Distribution of groups extracted for metrics"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	it.mDistMetricGroups = histo

	return &it, nil
}
