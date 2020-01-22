// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package carbonreceiver

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector/observability"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
)

type reporter struct {
	name          string
	spanName      string
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger // Used for generic debug logging
}

var _ (transport.Reporter) = (*reporter)(nil)

func newReporter(receiverName string, logger *zap.Logger) transport.Reporter {
	return &reporter{
		name:          receiverName,
		spanName:      receiverName + ".receiver",
		logger:        logger,
		sugaredLogger: logger.Sugar(),
	}
}

func (r *reporter) OnDataReceived(ctx context.Context) (context.Context, *trace.Span) {
	rcvCtx := observability.ContextWithReceiverName(ctx, r.name)
	span := trace.FromContext(rcvCtx)
	span.SetName(r.spanName)
	return rcvCtx, span
}

func (r *reporter) OnTranslationError(ctx context.Context, span *trace.Span, err error) {
	if err == nil {
		return
	}

	r.logger.Debug(
		"Carbon translation error",
		zap.String("receiver", r.name),
		zap.Error(err))

	// Using annotations since multiple translation errors can happen in the
	// same client message/request. The time itself is not relevant.
	span.Annotate([]trace.Attribute{
		trace.StringAttribute("error", err.Error())},
		"translation",
	)
}

func (r *reporter) OnMetricsProcessed(
	ctx context.Context,
	span *trace.Span,
	numReceivedTimeseries int,
	numInvalidTimeseries int,
	err error,
) {
	// TODO: the distinction between dropped and invalid is not precise.
	// 	It can be a valid metric that it was dropped due to lack of resources.
	// 	Will keep the ambiguity until standard observability metrics are
	// 	fixed.
	numDroppedTimeseries := numInvalidTimeseries
	if err != nil {
		r.logger.Debug(
			"Carbon receiver failed to be push metrics into pipeline",
			zap.String("receiver", r.name),
			zap.Int("numReceivedTimeseries", numReceivedTimeseries),
			zap.Int("numInvalidTimeseries", numInvalidTimeseries),
			zap.Error(err))

		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeUnknown,
			Message: err.Error(),
		})

		// In case of error assume that all time series were dropped.
		numDroppedTimeseries = numReceivedTimeseries
	}

	observability.RecordMetricsForMetricsReceiver(
		ctx, numReceivedTimeseries, numDroppedTimeseries)
}

func (r *reporter) OnDebugf(template string, args ...interface{}) {
	if r.logger.Check(zap.DebugLevel, "debug") != nil {
		r.sugaredLogger.Debugf(template, args...)
	}
}
