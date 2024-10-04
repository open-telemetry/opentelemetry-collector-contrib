// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/metadata"
)

// reporter struct implements the transport.Reporter interface to give consistent
// observability per Collector metric observability package.
type reporter struct {
	logger              *zap.Logger
	sugaredLogger       *zap.SugaredLogger // Used for generic debug logging
	receiverAttrSuccess metric.MeasurementOption
	receiverAttrFailure metric.MeasurementOption
	telemetryBuilder    *metadata.TelemetryBuilder
}

var (
	parseSuccessAttr = attribute.String("parse_success", "true")
	parseFailureAttr = attribute.String("parse_success", "false")
)

func newReporter(set receiver.Settings) (*reporter, error) {
	telemetryBuilder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return &reporter{
		logger:              set.Logger,
		sugaredLogger:       set.Logger.Sugar(),
		receiverAttrFailure: metric.WithAttributeSet(attribute.NewSet(attribute.String("receiver", set.ID.String()), parseFailureAttr)),
		receiverAttrSuccess: metric.WithAttributeSet(attribute.NewSet(attribute.String("receiver", set.ID.String()), parseSuccessAttr)),
		telemetryBuilder:    telemetryBuilder,
	}, nil
}

func (r *reporter) OnDebugf(template string, args ...any) {
	if r.logger.Check(zap.DebugLevel, "debug") != nil {
		r.sugaredLogger.Debugf(template, args...)
	}
}

func (r *reporter) RecordParseFailure() {
	r.telemetryBuilder.ReceiverReceivedStatsdMetrics.Add(context.Background(), 1, r.receiverAttrFailure)
}

func (r *reporter) RecordParseSuccess(count int64) {
	r.telemetryBuilder.ReceiverReceivedStatsdMetrics.Add(context.Background(), count, r.receiverAttrSuccess)
}
