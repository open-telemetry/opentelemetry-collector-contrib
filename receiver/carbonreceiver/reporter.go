// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/transport"
)

// reporter struct implements the transport.Reporter interface to give consistent
// observability per Collector metric observability package.
type reporter struct {
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger // Used for generic debug logging
	obsrecv       *receiverhelper.ObsReport
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

	return &reporter{
		logger:        set.Logger,
		sugaredLogger: set.Logger.Sugar(),
		obsrecv:       obsrecv,
	}, nil
}

// OnDataReceived is called when a message or request is received from
// a client. The returned context should be used in other calls to the same
// reporter instance. The caller code should include a call to end the
// returned span.
func (r *reporter) OnDataReceived(ctx context.Context) context.Context {
	return r.obsrecv.StartMetricsOp(ctx)
}

// OnTranslationError is used to report a translation error from original
// format to the internal format of the Collector. The context and span
// passed to it should be the ones returned by OnDataReceived.
func (r *reporter) OnTranslationError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	r.logger.Debug("Carbon translation error", zap.Error(err))

	trace.SpanFromContext(ctx).RecordError(err)
}

// OnMetricsProcessed is called when the received data is passed to next
// consumer on the pipeline. The context and span passed to it should be the
// ones returned by OnDataReceived. The error should be error returned by
// the next consumer - the reporter is expected to handle nil error too.
func (r *reporter) OnMetricsProcessed(
	ctx context.Context,
	numReceivedMetricPoints int,
	err error,
) {
	if err != nil {
		r.logger.Debug(
			"Carbon receiver failed to push metrics into pipeline",
			zap.Int("numReceivedMetricPoints", numReceivedMetricPoints),
			zap.Error(err))
	}

	r.obsrecv.EndMetricsOp(ctx, "carbon", numReceivedMetricPoints, err)
}

func (r *reporter) OnDebugf(template string, args ...any) {
	if r.logger.Check(zap.DebugLevel, "debug") != nil {
		r.sugaredLogger.Debugf(template, args...)
	}
}
