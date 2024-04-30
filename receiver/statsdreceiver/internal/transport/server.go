// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import (
	"errors"
	"net"

	"go.opentelemetry.io/collector/consumer"
)

var errNilListenAndServeParameters = errors.New("no parameter of ListenAndServe can be nil")

// Server abstracts the type of transport being used and offer an
// interface to handle serving clients over that transport.
type Server interface {
	// ListenAndServe is a blocking call that starts to listen for client messages
	// on the specific transport, and prepares the message to be processed by
	// the Parser and passed to the next consumer.
	ListenAndServe(
		mc consumer.Metrics,
		r Reporter,
		transferChan chan<- Metric,
	) error

	// Close stops any running ListenAndServe, however, it waits for any
	// data already received to be parsed and sent to the next consumer.
	Close() error
}

type Metric struct {
	Raw  string
	Addr net.Addr
}

// Reporter is used to report (via zPages, logs, metrics, etc) the events
// happening when the Server is receiving and processing data.
type Reporter interface {
	// OnDebugf allows less structured reporting for debugging scenarios.
	OnDebugf(
		template string,
		args ...any)
}
