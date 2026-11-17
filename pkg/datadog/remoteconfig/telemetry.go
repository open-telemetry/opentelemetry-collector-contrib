// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

package remoteconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/remoteconfig"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type serviceReporter struct {
	rateLimited  metric.Int64Counter
	timedOut     metric.Int64Counter
	connected    metric.Int64Counter
	disconnected metric.Int64Counter
	active       metric.Int64Gauge
	tracked      metric.Int64Gauge
	attributes   metric.MeasurementOption
}

func newServiceReporter(set extension.Settings) (*serviceReporter, error) {
	meter := set.MeterProvider.Meter("datadog.extension.remote_config")
	r := &serviceReporter{attributes: metric.WithAttributes(attribute.String("extension", set.ID.String()))}
	var errs [6]error
	r.rateLimited, errs[0] = meter.Int64Counter("datadog.remote_config.cache_bypass.rate_limited", metric.WithDescription("Cache-bypass refreshes rejected by the Agent RC rate limiter"), metric.WithUnit("{refresh}"))
	r.timedOut, errs[1] = meter.Int64Counter("datadog.remote_config.cache_bypass.timed_out", metric.WithDescription("Client requests whose cold-client refresh timed out"), metric.WithUnit("{request}"))
	r.connected, errs[2] = meter.Int64Counter("datadog.remote_config.subscriptions.connected", metric.WithUnit("{subscription}"))
	r.disconnected, errs[3] = meter.Int64Counter("datadog.remote_config.subscriptions.disconnected", metric.WithUnit("{subscription}"))
	r.active, errs[4] = meter.Int64Gauge("datadog.remote_config.subscriptions.active", metric.WithUnit("{subscription}"))
	r.tracked, errs[5] = meter.Int64Gauge("datadog.remote_config.subscriptions.clients", metric.WithUnit("{client}"))
	return r, errors.Join(errs[:]...)
}

func (r *serviceReporter) IncRateLimit() {
	r.rateLimited.Add(context.Background(), 1, r.attributes)
}

func (r *serviceReporter) IncTimeout() {
	r.timedOut.Add(context.Background(), 1, r.attributes)
}

func (r *serviceReporter) IncConfigSubscriptionsConnectedCounter() {
	r.connected.Add(context.Background(), 1, r.attributes)
}

func (r *serviceReporter) IncConfigSubscriptionsDisconnectedCounter() {
	r.disconnected.Add(context.Background(), 1, r.attributes)
}

func (r *serviceReporter) SetConfigSubscriptionsActive(n int) {
	r.active.Record(context.Background(), int64(n), r.attributes)
}

func (r *serviceReporter) SetConfigSubscriptionClientsTracked(n int) {
	r.tracked.Record(context.Background(), int64(n), r.attributes)
}
