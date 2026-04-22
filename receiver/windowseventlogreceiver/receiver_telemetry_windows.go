// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/metadata"
)

type receiverWindowsTelemetry struct {
	tb  *metadata.TelemetryBuilder
	cfg MetricsConfig
}

func (r *receiverWindowsTelemetry) RecordEventSize(ctx context.Context, channel string, sizeBytes int) {
	if !r.cfg.ReceiverWindowsEventLogEventSize.Enabled {
		return
	}
	r.tb.ReceiverWindowsEventLogEventSize.Record(ctx, int64(sizeBytes),
		metric.WithAttributes(attribute.String("channel", channel)))
}

func (r *receiverWindowsTelemetry) RecordChannelSize(ctx context.Context, channel string, size int64) {
	if !r.cfg.ReceiverWindowsEventLogChannelSize.Enabled {
		return
	}
	r.tb.ReceiverWindowsEventLogChannelSize.Record(ctx, size,
		metric.WithAttributes(attribute.String("channel", channel)))
}

func (r *receiverWindowsTelemetry) RecordMissedEvents(ctx context.Context, channel string, count int64) {
	if !r.cfg.ReceiverWindowsEventLogMissedEvents.Enabled {
		return
	}
	r.tb.ReceiverWindowsEventLogMissedEvents.Add(ctx, count,
		metric.WithAttributes(attribute.String("channel", channel)))
}

func (r *receiverWindowsTelemetry) RecordBatchSize(ctx context.Context, channel string, count int64) {
	if !r.cfg.ReceiverWindowsEventLogBatchSize.Enabled {
		return
	}
	r.tb.ReceiverWindowsEventLogBatchSize.Record(ctx, count,
		metric.WithAttributes(attribute.String("channel", channel)))
}
