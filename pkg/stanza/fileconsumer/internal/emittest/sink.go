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

type Sink struct {
	emitChan chan emit.Token
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
	emitChan := make(chan emit.Token, cfg.emitChanLen)
	return &Sink{
		emitChan: emitChan,
		timeout:  cfg.timeout,
		Callback: func(ctx context.Context, tokens [][]byte, attributes map[string]any, _ int64, _ []int64) error {
			for _, token := range tokens {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case emitChan <- emit.NewToken(token, attributes):
				}
			}
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
	for range n {
		select {
		case token := <-s.emitChan:
			emitChan = append(emitChan, token.Body)
		case <-time.After(s.timeout):
			assert.Fail(t, "Timed out waiting for message")
			return nil
		}
	}
	return emitChan
}

func (s *Sink) NextCall(t *testing.T) ([]byte, map[string]any) {
	select {
	case token := <-s.emitChan:
		return token.Body, token.Attributes
	case <-time.After(s.timeout):
		assert.Fail(t, "Timed out waiting for message")
		return nil, nil
	}
}

func (s *Sink) ExpectToken(t *testing.T, expected []byte) {
	select {
	case token := <-s.emitChan:
		assert.Equal(t, expected, token.Body)
	case <-time.After(s.timeout):
		assert.Fail(t, fmt.Sprintf("Timed out waiting for token: %s", expected))
	}
}

func (s *Sink) ExpectTokens(t *testing.T, expected ...[]byte) {
	actual := make([][]byte, 0, len(expected))
	for i := range expected {
		select {
		case token := <-s.emitChan:
			actual = append(actual, token.Body)
		case <-time.After(s.timeout):
			assert.Fail(t, fmt.Sprintf("timeout: expected: %d, actual: %d", len(expected), i))
			return
		}
	}
	require.ElementsMatchf(t, expected, actual, "expected: %v, actual: %v", expected, actual)
}

func (s *Sink) ExpectCall(t *testing.T, expected []byte, attrs map[string]any) {
	select {
	case token := <-s.emitChan:
		assert.Equal(t, expected, token.Body)
		assert.Equal(t, attrs, token.Attributes)
	case <-time.After(s.timeout):
		assert.Fail(t, fmt.Sprintf("Timed out waiting for token: %s", expected))
	}
}

func (s *Sink) ExpectCalls(t *testing.T, expected ...emit.Token) {
	actual := make([]emit.Token, 0, len(expected))
	for i := range expected {
		select {
		case call := <-s.emitChan:
			actual = append(actual, call)
		case <-time.After(s.timeout):
			assert.Fail(t, fmt.Sprintf("timeout: expected: %d, actual: %d", len(expected), i))
			return
		}
	}
	require.ElementsMatch(t, expected, actual)
}

func (s *Sink) ExpectNoCalls(t *testing.T) {
	s.ExpectNoCallsUntil(t, 200*time.Millisecond)
}

func (s *Sink) ExpectNoCallsUntil(t *testing.T, d time.Duration) {
	select {
	case c := <-s.emitChan:
		assert.Fail(t, "Received unexpected message", "Message: %s", c.Body)
	case <-time.After(d):
	}
}
