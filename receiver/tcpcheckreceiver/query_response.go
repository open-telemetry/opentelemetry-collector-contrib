// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
)

var errExpectMismatch = errors.New("response did not match expectation")

func (cfg *TargetConfig) validateQueryResponse() error {
	if cfg.MaxReadSize < 0 {
		return errors.New("max_read_size must not be negative")
	}
	if len(cfg.QueryResponse) > 0 && cfg.DialerConfig.Timeout <= 0 {
		return errors.New("dialer.timeout must be positive when query_response is configured")
	}
	for i := range cfg.QueryResponse {
		if err := cfg.QueryResponse[i].validate(); err != nil {
			return &queryResponseError{step: i, err: err}
		}
	}
	return nil
}

func (q *QueryResponse) validate() error {
	q.expectRegexp = nil
	q.sendData = nil
	q.expectData = nil
	if q.Send == "" && q.SendBytes == "" && q.Expect == "" && q.ExpectBytes == "" {
		return errors.New("step must contain at least one operation")
	}
	if q.Send != "" && q.SendBytes != "" {
		return errors.New("send and send_bytes are mutually exclusive")
	}
	if q.Expect != "" && q.ExpectBytes != "" {
		return errors.New("expect and expect_bytes are mutually exclusive")
	}
	var err error
	if q.Expect != "" {
		q.expectRegexp, err = regexp.Compile(q.Expect)
		if err != nil {
			return fmt.Errorf("invalid expect regular expression: %w", err)
		}
	}
	if q.SendBytes != "" {
		q.sendData, err = base64.StdEncoding.DecodeString(q.SendBytes)
		if err != nil {
			return fmt.Errorf("invalid send_bytes base64: %w", err)
		}
	}
	if q.ExpectBytes != "" {
		q.expectData, err = base64.StdEncoding.DecodeString(q.ExpectBytes)
		if err != nil {
			return fmt.Errorf("invalid expect_bytes base64: %w", err)
		}
		if len(q.expectData) == 0 {
			return errors.New("expect_bytes must decode to at least one byte")
		}
	}
	return nil
}

// queryResponseError identifies the failing step without exposing response data or patterns.
type queryResponseError struct {
	step int
	err  error
}

func (e *queryResponseError) Error() string {
	return fmt.Sprintf("query_response[%d]: %v", e.step, e.err)
}

func (e *queryResponseError) Unwrap() error {
	return e.err
}

func runQueryResponse(ctx context.Context, conn net.Conn, cfg *TargetConfig) error {
	maxReadSize := cfg.MaxReadSize
	if maxReadSize == 0 {
		maxReadSize = defaultMaxReadSize
	}
	r := responseReader{reader: conn, maxReadSize: maxReadSize}
	for i := range cfg.QueryResponse {
		step := &cfg.QueryResponse[i]
		if err := ctx.Err(); err != nil {
			return &queryResponseError{step: i, err: err}
		}
		send := []byte(step.Send)
		var err error
		if step.Expect != "" || step.ExpectBytes != "" {
			send, err = r.expect(step)
			if err != nil {
				if ctx.Err() != nil {
					err = ctx.Err()
				}
				return &queryResponseError{step: i, err: err}
			}
		}
		if step.SendBytes != "" {
			send = step.sendData
		}
		if len(send) > 0 {
			_, err = io.Copy(conn, bytes.NewReader(send))
			if err != nil {
				if ctx.Err() != nil {
					err = ctx.Err()
				}
				return &queryResponseError{step: i, err: err}
			}
		}
	}
	return nil
}

type responseReader struct {
	reader      io.Reader
	buffer      []byte
	readErr     error
	maxReadSize int
}

func (r *responseReader) expect(step *QueryResponse) ([]byte, error) {
	var chunk [4096]byte
	for {
		end := -1
		send := []byte(step.Send)
		if step.expectRegexp != nil {
			if match := step.expectRegexp.FindSubmatchIndex(r.buffer); match != nil {
				end = match[1]
				send = step.expectRegexp.ExpandString(nil, step.Send, string(r.buffer), match)
			}
		} else if index := bytes.Index(r.buffer, step.expectData); index >= 0 {
			end = index + len(step.expectData)
		}
		if end >= 0 {
			// Keep bytes read beyond this match for the next expect operation.
			r.buffer = r.buffer[end:]
			return send, nil
		}
		if r.readErr != nil {
			if errors.Is(r.readErr, io.EOF) {
				return nil, errExpectMismatch
			}
			return nil, r.readErr
		}
		remaining := r.maxReadSize - len(r.buffer)
		if remaining <= 0 {
			return nil, fmt.Errorf("%w: max_read_size exceeded", errExpectMismatch)
		}
		n, err := r.reader.Read(chunk[:min(len(chunk), remaining)])
		r.buffer = append(r.buffer, chunk[:n]...)
		r.readErr = err
	}
}
