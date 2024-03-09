// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"
)

// reporter struct implements the transport.Reporter interface to give consistent
// observability per Collector metric observability package.
type reporter struct {
	logger               *zap.Logger
	sugaredLogger        *zap.SugaredLogger // Used for generic debug logging
	obsrecv              *receiverhelper.ObsReport
	staticAttrs          []attribute.KeyValue
	acceptedMetricPoints metric.Int64Counter
	refusedMetricPoints  metric.Int64Counter
}

var _ transport.Reporter = (*reporter)(nil)

func newReporter(set receiver.CreateSettings) (transport.Reporter, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "tcp",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	r := &reporter{
		logger:        set.Logger,
		sugaredLogger: set.Logger.Sugar(),
		obsrecv:       obsrecv,
		staticAttrs: []attribute.KeyValue{
			attribute.String("receiver", set.ID.String()),
		},
	}

	// See https://github.com/open-telemetry/opentelemetry-collector/blob/241334609fc47927b4a8533dfca28e0f65dad9fe/receiver/receiverhelper/obsreport.go#L104
	// for the metric naming conventions

	r.acceptedMetricPoints, err = metadata.Meter(set.TelemetrySettings).Int64Counter(
		"receiver/accepted_metric_points",
		metric.WithDescription("Number of metric data points accepted"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	r.refusedMetricPoints, err = metadata.Meter(set.TelemetrySettings).Int64Counter(
		"receiver/refused_metric_points",
		metric.WithDescription("Number of metric data points refused"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *reporter) OnDebugf(template string, args ...any) {
	if r.logger.Check(zap.DebugLevel, "debug") != nil {
		r.sugaredLogger.Debugf(template, args...)
	}
}

func (r *reporter) RecordAcceptedMetric() {
	r.acceptedMetricPoints.Add(context.Background(), 1, metric.WithAttributes(r.staticAttrs...))
}

func (r *reporter) RecordRefusedMetric() {
	r.refusedMetricPoints.Add(context.Background(), 1, metric.WithAttributes(r.staticAttrs...))
}
