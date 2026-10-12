// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const readChunkSize = 4096

var (
	errResponseTooLarge = errors.New("response exceeded max_response_bytes without matching")
	errConnectionClosed = errors.New("connection closed before the expected response was received")
)

// queryResponseError reports which query_response step failed. The response
// data and the configured pattern are intentionally not part of the message.
type queryResponseError struct {
	step int
	err  error
}

func (e *queryResponseError) Error() string {
	return fmt.Sprintf("query_response step %d failed: %v", e.step, e.err)
}

func (e *queryResponseError) Unwrap() error {
	return e.err
}

// exchangeDeadline returns the deadline that bounds the whole check (dial plus
// query/response exchange): the earliest of the context deadline and
// start+timeout. A zero time means no deadline.
func exchangeDeadline(ctx context.Context, start time.Time, timeout time.Duration) time.Time {
	var deadline time.Time
	if timeout > 0 {
		deadline = start.Add(timeout)
	}
	if ctxDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || ctxDeadline.Before(deadline)) {
		deadline = ctxDeadline
	}
	return deadline
}

// runQueryResponse executes the steps in order on conn. The first failing step
// aborts the exchange and is reported through a queryResponseError.
func runQueryResponse(ctx context.Context, conn net.Conn, steps []compiledStep, maxResponseBytes int, deadline time.Time) error {
	if len(steps) == 0 {
		return nil
	}
	if !deadline.IsZero() {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}
	// Unblock any pending read or write if the scrape context is cancelled.
	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stop()

	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	r := &responseReader{
		conn:  conn,
		max:   maxResponseBytes,
		chunk: make([]byte, readChunkSize),
	}
	for i := range steps {
		if err := r.runStep(&steps[i]); err != nil {
			return &queryResponseError{step: i, err: err}
		}
	}
	return nil
}

// responseReader accumulates data read from the connection. Data consumed by a
// matching expect step is dropped; anything after the match stays buffered for
// the following steps.
type responseReader struct {
	conn  net.Conn
	buf   []byte
	max   int
	chunk []byte
}

func (r *responseReader) runStep(step *compiledStep) error {
	send := step.send

	switch {
	case step.expectRe != nil:
		match, err := r.readUntil(step.expectRe.FindSubmatchIndex)
		if err != nil {
			return err
		}
		if step.sendIsTmpl {
			send = step.expectRe.Expand(nil, step.send, r.buf, match)
		}
		r.consume(match[1])
	case step.expectBytes != nil:
		match, err := r.readUntil(func(b []byte) []int {
			idx := bytes.Index(b, step.expectBytes)
			if idx < 0 {
				return nil
			}
			return []int{idx, idx + len(step.expectBytes)}
		})
		if err != nil {
			return err
		}
		r.consume(match[1])
	}

	if len(send) > 0 {
		if _, err := r.conn.Write(send); err != nil {
			return fmt.Errorf("send failed: %w", err)
		}
	}
	return nil
}

// readUntil reads from the connection until matcher reports a match on the
// buffered data and returns the match indices relative to r.buf.
func (r *responseReader) readUntil(matcher func([]byte) []int) ([]int, error) {
	var readErr error
	for {
		if match := matcher(r.buf); match != nil {
			return match, nil
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil, errConnectionClosed
			}
			return nil, fmt.Errorf("read failed: %w", readErr)
		}
		if len(r.buf) >= r.max {
			return nil, errResponseTooLarge
		}

		var n int
		n, readErr = r.conn.Read(r.chunk[:min(len(r.chunk), r.max-len(r.buf))])
		r.buf = append(r.buf, r.chunk[:n]...)
	}
}

func (r *responseReader) consume(n int) {
	remaining := copy(r.buf, r.buf[n:])
	r.buf = r.buf[:remaining]
}
