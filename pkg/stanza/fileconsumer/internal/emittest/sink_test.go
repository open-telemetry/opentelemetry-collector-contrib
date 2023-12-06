// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextToken(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		token := s.NextToken(t)
		assert.Equal(t, c.token, token)
	}
}

func TestNextTokenTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		token := s.NextToken(t)
		assert.Equal(t, c.token, token)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.NextToken(tt)
	assert.True(t, tt.Failed())
}

func TestNextTokens(t *testing.T) {
	s, testCalls := sinkTest(t)
	for i := 0; i < 5; i++ {
		tokens := s.NextTokens(t, 2)
		assert.Equal(t, testCalls[2*i].token, tokens[0])
		assert.Equal(t, testCalls[2*i+1].token, tokens[1])
	}
}

func TestNextTokensTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for i := 0; i < 5; i++ {
		tokens := s.NextTokens(t, 2)
		assert.Equal(t, testCalls[2*i].token, tokens[0])
		assert.Equal(t, testCalls[2*i+1].token, tokens[1])
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.NextTokens(tt, 2)
	assert.True(t, tt.Failed())
}

func TestNextCall(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		token, attributes := s.NextCall(t)
		require.Equal(t, c.token, token)
		require.Equal(t, c.attrs, attributes)
	}
}

func TestNextCallTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		token, attributes := s.NextCall(t)
		require.Equal(t, c.token, token)
		require.Equal(t, c.attrs, attributes)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.NextCall(tt)
	assert.True(t, tt.Failed())
}

func TestExpectToken(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		s.ExpectToken(t, c.token)
	}
}

func TestExpectTokenTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		s.ExpectToken(t, c.token)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectToken(tt, []byte("foo"))
	assert.True(t, tt.Failed())
}

func TestExpectTokens(t *testing.T) {
	s, testCalls := sinkTest(t)
	for i := 0; i < 5; i++ {
		s.ExpectTokens(t, testCalls[2*i].token, testCalls[2*i+1].token)
	}
}

func TestExpectTokensTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for i := 0; i < 5; i++ {
		s.ExpectTokens(t, testCalls[2*i].token, testCalls[2*i+1].token)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectTokens(tt, []byte("foo"), []byte("bar"))
	assert.True(t, tt.Failed())
}

func TestExpectCall(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		s.ExpectCall(t, c.token, c.attrs)
	}
}

func TestExpectCallTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		s.ExpectCall(t, c.token, c.attrs)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectCall(tt, []byte("foo"), nil)
	assert.True(t, tt.Failed())
}

func TestExpectNoCalls(t *testing.T) {
	s, _ := sinkTest(t)
	s.NextTokens(t, 10) // drain the channel
	s.ExpectNoCalls(t)
}

func TestExpectNoCallsFailure(t *testing.T) {
	s, _ := sinkTest(t)
	s.NextTokens(t, 9) // partially drain the channel

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectNoCalls(tt)
	assert.True(t, tt.Failed())
}

func TestWithCallBuffer(t *testing.T) {
	s, testCalls := sinkTest(t, WithCallBuffer(5))
	for i := 0; i < 10; i++ {
		s.ExpectCall(t, testCalls[i].token, testCalls[i].attrs)
	}
}

func sinkTest(t *testing.T, opts ...SinkOpt) (*Sink, []*call) {
	s := NewSink(opts...)
	testCalls := make([]*call, 0, 10)
	for i := 0; i < 10; i++ {
		testCalls = append(testCalls, &call{
			token: []byte(fmt.Sprintf("token-%d", i)),
			attrs: map[string]any{
				"key": fmt.Sprintf("value-%d", i),
			},
		})
	}
	go func() {
		for _, c := range testCalls {
			require.NoError(t, s.Callback(context.Background(), c.token, c.attrs))
		}
	}()
	return s, testCalls
}
