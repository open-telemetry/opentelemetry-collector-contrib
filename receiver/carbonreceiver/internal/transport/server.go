// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/transport"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

var errNilListenAndServeParameters = errors.New(
	"no parameter of ListenAndServe can be nil")

// Server abstracts the type of transport being used and offer an
// interface to handle serving clients over that transport.
type Server interface {
	// ListenAndServe is a blocking call that starts to listen for client messages
	// on the specific transport, and prepares the message to be processed by
	// the Parser and passed to the next consumer.
	//
	// Returns net.ErrClosed when closed.
	ListenAndServe(
		p protocol.Parser,
		mc consumer.Metrics,
		r Reporter,
	) error

	// Close stops any running ListenAndServe, however, it waits for any
	// data already received to be parsed and sent to the next consumer.
	Close() error
}

// Reporter is used to report (via zPages, logs, metrics, etc) the events
// happening when the Server is receiving and processing data.
type Reporter interface {
	// OnDataReceived is called when a message or request is received from
	// a client. The returned context should be used in other calls to the same
	// reporter instance. The caller code should include a call to end the
	// returned span.
	OnDataReceived(ctx context.Context) context.Context

	// OnTranslationError is used to report a translation error from original
	// format to the internal format of the Collector. The context
	// passed to it should be the ones returned by OnDataReceived.
	OnTranslationError(ctx context.Context, err error)

	// OnMetricsProcessed is called when the received data is passed to next
	// consumer on the pipeline. The context passed to it should be the
	// one returned by OnDataReceived. The error should be error returned by
	// the next consumer - the reporter is expected to handle nil error too.
	OnMetricsProcessed(
		ctx context.Context,
		numReceivedMetricPoints int,
		err error)

	// OnDebugf allows less structured reporting for debugging scenarios.
	OnDebugf(
		template string,
		args ...any)
}
