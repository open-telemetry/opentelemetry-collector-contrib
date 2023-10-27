// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/metric"
)

type observability struct {
	mErrors            metric.Int64Counter
	mRequestsReceived  metric.Int64Counter
	mMetricsReceived   metric.Int64Counter
	mEventsReceived    metric.Int64Counter
	mBlankDefaultAttrs metric.Int64Counter
}

func initMetrics(telemetrySettings component.TelemetrySettings) (*observability, error) {
	meter := telemetrySettings.MeterProvider.Meter("collectd")

	mErrors, err := meter.Int64Counter("errors", metric.WithUnit("1"), metric.WithDescription("Errors encountered during processing of collectd requests"))
	if err != nil {
		return nil, err
	}
	mRequestsReceived, err := meter.Int64Counter("requests_received", metric.WithUnit("1"), metric.WithDescription("Number of total requests received"))
	if err != nil {
		return nil, err
	}
	mMetricsReceived, err := meter.Int64Counter("metrics_received", metric.WithUnit("1"), metric.WithDescription("Number of metrics received"))
	if err != nil {
		return nil, err
	}
	mEventsReceived, err := meter.Int64Counter("events_received", metric.WithUnit("1"), metric.WithDescription("Number of events received"))
	if err != nil {
		return nil, err
	}
	mBlankDefaultAttrs, err := meter.Int64Counter("blank_default_attrs", metric.WithUnit("1"), metric.WithDescription("Number of blank default attributes received"))
	if err != nil {
		return nil, err
	}
	return &observability{
		mErrors:            mErrors,
		mRequestsReceived:  mRequestsReceived,
		mMetricsReceived:   mMetricsReceived,
		mEventsReceived:    mEventsReceived,
		mBlankDefaultAttrs: mBlankDefaultAttrs,
	}, nil
}

func (o *observability) recordRequestErrors() {
	o.mErrors.Add(context.Background(), 1)
}

func (o *observability) recordRequestReceived() {
	o.mRequestsReceived.Add(context.Background(), 1)
}

func (o *observability) recordMetricsReceived() {
	o.mMetricsReceived.Add(context.Background(), 1)
}

func (o *observability) recordEventsReceived() {
	o.mEventsReceived.Add(context.Background(), 1)
}

func (o *observability) recordDefaultBlankAttrs() {
	o.mBlankDefaultAttrs.Add(context.Background(), 1)
}
