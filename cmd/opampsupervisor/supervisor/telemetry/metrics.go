// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	CollectorHealthStatusMetric   = "supervisor.agent.health_status"
	CollectorFallbackStatusMetric = "supervisor.agent.fallback_status"
	OpAMPConnectionStatusMetric   = "supervisor.opamp.connection.status"
	OpAMPConnectionAttemptsMetric = "supervisor.opamp.connection.attempts"
	OpAMPDisconnectsMetric        = "supervisor.opamp.disconnects"
	OpAMPConnectionDurationMetric = "supervisor.opamp.connection.duration"
	OpAMPMessagesMetric           = "supervisor.opamp.messages"
	OpAMPPayloadBytesMetric       = "supervisor.opamp.network.io"
)

type Metrics struct {
	collectorHealthStatusMetric   metric.Int64UpDownCounter
	collectorFallbackStatusMetric metric.Int64UpDownCounter
	opampConnectionStatusMetric   metric.Int64UpDownCounter
	opampConnectionAttemptsMetric metric.Int64Counter
	opampDisconnectsMetric        metric.Int64Counter
	opampConnectionDurationMetric metric.Float64Histogram
	opampMessagesMetric           metric.Int64Counter
	opampPayloadBytesMetric       metric.Int64Counter
	healthStatus                  bool
	fallbackStatus                bool
	connectionStartedAt           time.Time
	mu                            sync.Mutex
}

func NewMetrics(meterProvider metric.MeterProvider) (*Metrics, error) {
	meter := meterProvider.Meter("opamp-supervisor")

	healthStatus, err := meter.Int64UpDownCounter(
		CollectorHealthStatusMetric,
		metric.WithDescription("Current health status of the collector (1=healthy, 0=unhealthy)"),
	)
	if err != nil {
		return nil, err
	}

	fallbackStatus, err := meter.Int64UpDownCounter(
		CollectorFallbackStatusMetric,
		metric.WithDescription("Current fallback status of the collector (1=fallback active, 0=fallback inactive)"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize metrics to 0 to ensure they are exported
	healthStatus.Add(context.Background(), 0)
	opampConnectionStatus, err := meter.Int64UpDownCounter(
		OpAMPConnectionStatusMetric,
		metric.WithUnit("{connection}"),
		metric.WithDescription("Current OpAMP connection status (1=connected, 0=disconnected)"),
	)
	if err != nil {
		return nil, err
	}
	connectionAttempts, err := meter.Int64Counter(
		OpAMPConnectionAttemptsMetric,
		metric.WithUnit("{attempt}"),
		metric.WithDescription("OpAMP connection attempts"),
	)
	if err != nil {
		return nil, err
	}
	disconnects, err := meter.Int64Counter(
		OpAMPDisconnectsMetric,
		metric.WithUnit("{disconnect}"),
		metric.WithDescription("Lost established OpAMP connections"),
	)
	if err != nil {
		return nil, err
	}
	connectionDuration, err := meter.Float64Histogram(
		OpAMPConnectionDurationMetric,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of established OpAMP connections"),
	)
	if err != nil {
		return nil, err
	}
	messages, err := meter.Int64Counter(
		OpAMPMessagesMetric,
		metric.WithUnit("{message}"),
		metric.WithDescription("OpAMP messages exchanged"),
	)
	if err != nil {
		return nil, err
	}
	payloadBytes, err := meter.Int64Counter(
		OpAMPPayloadBytesMetric,
		metric.WithUnit("By"),
		metric.WithDescription("Protobuf payload bytes exchanged through OpAMP"),
	)
	if err != nil {
		return nil, err
	}
	return &Metrics{
		collectorHealthStatusMetric:   healthStatus,
		collectorFallbackStatusMetric: fallbackStatus,
		opampConnectionStatusMetric:   opampConnectionStatus,
		opampConnectionAttemptsMetric: connectionAttempts,
		opampDisconnectsMetric:        disconnects,
		opampConnectionDurationMetric: connectionDuration,
		opampMessagesMetric:           messages,
		opampPayloadBytesMetric:       payloadBytes,
		healthStatus:                  false,
	}, nil
}

func (m *Metrics) RecordOpAMPConnect(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connectionStartedAt.IsZero() {
		return
	}
	m.opampConnectionStatusMetric.Add(ctx, 1)
	m.connectionStartedAt = time.Now()
}

func (m *Metrics) RecordOpAMPConnectAttempt(ctx context.Context) {
	m.opampConnectionAttemptsMetric.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", "success")))
}

func (m *Metrics) RecordOpAMPConnectFailed(ctx context.Context) {
	m.opampConnectionAttemptsMetric.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", "failure")))
}

func (m *Metrics) RecordOpAMPDisconnect(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectionStartedAt.IsZero() {
		return
	}
	m.opampConnectionStatusMetric.Add(ctx, -1)
	m.opampDisconnectsMetric.Add(ctx, 1)
	m.opampConnectionDurationMetric.Record(ctx, time.Since(m.connectionStartedAt).Seconds())
	m.connectionStartedAt = time.Time{}
}

func (m *Metrics) RecordOpAMPMessageSent(ctx context.Context, size int) {
	m.recordOpAMPMessage(ctx, "transmit", size)
}

func (m *Metrics) RecordOpAMPMessageReceived(ctx context.Context, size int) {
	m.recordOpAMPMessage(ctx, "receive", size)
}

func (m *Metrics) recordOpAMPMessage(ctx context.Context, direction string, size int) {
	attrs := metric.WithAttributes(attribute.String("network.io.direction", direction))
	m.opampMessagesMetric.Add(ctx, 1, attrs)
	m.opampPayloadBytesMetric.Add(ctx, int64(size), attrs)
}
func (m *Metrics) SetCollectorHealthStatus(ctx context.Context, healthy bool) {
	// Only update the metric if the health status has changed
	if !m.healthStatus && healthy {
		m.collectorHealthStatusMetric.Add(ctx, 1)
		m.healthStatus = healthy
	} else if m.healthStatus && !healthy {
		m.collectorHealthStatusMetric.Add(ctx, -1)
		m.healthStatus = healthy
	}
}

func (m *Metrics) SetCollectorFallbackStatus(ctx context.Context, fallbackActive bool) {
	// Only update the metric if the fallback status has changed
	if !m.fallbackStatus && fallbackActive {
		m.collectorFallbackStatusMetric.Add(ctx, 1)
		m.fallbackStatus = fallbackActive
	} else if m.fallbackStatus && !fallbackActive {
		m.collectorFallbackStatusMetric.Add(ctx, -1)
		m.fallbackStatus = fallbackActive
	}
}
