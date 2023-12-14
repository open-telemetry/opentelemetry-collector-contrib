// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
)

type sinkCfg struct {
	emitChanLen int
	timeout     time.Duration
}

type SinkOpt func(*sinkCfg)

type call struct {
	token []byte
	attrs map[string]any
}

type Sink struct {
	emitChan chan *call
	timeout  time.Duration
	emit.Callback
}

func WithCallBuffer(n int) SinkOpt {
	return func(cfg *sinkCfg) {
		cfg.emitChanLen = n
	}
}

func WithTimeout(d time.Duration) SinkOpt {
	return func(cfg *sinkCfg) {
		cfg.timeout = d
	}
}

func NewSink(opts ...SinkOpt) *Sink {
	cfg := &sinkCfg{
		emitChanLen: 100,
		timeout:     3 * time.Second,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	emitChan := make(chan *call, cfg.emitChanLen)
	return &Sink{
		emitChan: emitChan,
		timeout:  cfg.timeout,
		Callback: func(_ context.Context, token []byte, attrs map[string]any) error {
			copied := make([]byte, len(token))
			copy(copied, token)
			emitChan <- &call{copied, attrs}
			return nil
		},
	}
}

func (s *Sink) NextToken(t *testing.T) []byte {
	token, _ := s.NextCall(t)
	return token
}

func (s *Sink) NextTokens(t *testing.T, n int) [][]byte {
	emitChan := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		select {
		case call := <-s.emitChan:
			emitChan = append(emitChan, call.token)
		case <-time.After(s.timeout):
			assert.Fail(t, "Timed out waiting for message")
			return nil
		}
	}
	return emitChan
}

func (s *Sink) NextCall(t *testing.T) ([]byte, map[string]any) {
	select {
	case c := <-s.emitChan:
		return c.token, c.attrs
	case <-time.After(s.timeout):
		assert.Fail(t, "Timed out waiting for message")
		return nil, nil
	}
}

func (s *Sink) ExpectToken(t *testing.T, expected []byte) {
	select {
	case call := <-s.emitChan:
		assert.Equal(t, expected, call.token)
	case <-time.After(s.timeout):
		assert.Fail(t, fmt.Sprintf("Timed out waiting for token: %s", expected))
	}
}

func (s *Sink) ExpectTokens(t *testing.T, expected ...[]byte) {
	actual := make([][]byte, 0, len(expected))
	for i := 0; i < len(expected); i++ {
		select {
		case call := <-s.emitChan:
			actual = append(actual, call.token)
		case <-time.After(s.timeout):
			assert.Fail(t, "Timed out waiting for message")
			return
		}
	}
	require.ElementsMatch(t, expected, actual)
}

func (s *Sink) ExpectCall(t *testing.T, expected []byte, attrs map[string]any) {
	select {
	case c := <-s.emitChan:
		assert.Equal(t, expected, c.token)
		assert.Equal(t, attrs, c.attrs)
	case <-time.After(s.timeout):
		assert.Fail(t, fmt.Sprintf("Timed out waiting for token: %s", expected))
	}
}

func (s *Sink) ExpectNoCalls(t *testing.T) {
	s.ExpectNoCallsUntil(t, 200*time.Millisecond)
}

func (s *Sink) ExpectNoCallsUntil(t *testing.T, d time.Duration) {
	select {
	case c := <-s.emitChan:
		assert.Fail(t, "Received unexpected message", "Message: %s", c.token)
	case <-time.After(d):
	}
}
