// Copyright The OpenTelemetry Authors
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

package statsdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver"

import (
	"context"

	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport"
)

// reporter struct implements the transport.Reporter interface to give consistent
// observability per Collector metric observability package.
type reporter struct {
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger // Used for generic debug logging
	obsrecv       *obsreport.Receiver
}

var _ transport.Reporter = (*reporter)(nil)

func newReporter(set receiver.CreateSettings) (transport.Reporter, error) {
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
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

	r.logger.Debug("StatsD translation error", zap.Error(err))

	// Using annotations since multiple translation errors can happen in the
	// same client message/request. The time itself is not relevant.
	span := trace.FromContext(ctx)
	span.Annotate([]trace.Attribute{
		trace.StringAttribute("error", err.Error())},
		"translation",
	)
}

// OnMetricsProcessed is called when the received data is passed to next
// consumer on the pipeline. The context and span passed to it should be the
// ones returned by OnDataReceived. The error should be error returned by
// the next consumer - the reporter is expected to handle nil error too.
func (r *reporter) OnMetricsProcessed(
	ctx context.Context,
	numReceivedMessages int,
	err error,
) {
	if err != nil {
		r.logger.Debug(
			"StatsD receiver failed to push metrics into pipeline",
			zap.Int("numReceivedMessages", numReceivedMessages),
			zap.Error(err))

		span := trace.FromContext(ctx)
		span.SetStatus(trace.Status{
			Code:    trace.StatusCodeUnknown,
			Message: err.Error(),
		})
	}

	r.obsrecv.EndMetricsOp(ctx, "statsd", numReceivedMessages, err)
}

func (r *reporter) OnDebugf(template string, args ...interface{}) {
	if r.logger.Check(zap.DebugLevel, "debug") != nil {
		r.sugaredLogger.Debugf(template, args...)
	}
}
