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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
)

func TestNextToken(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		token := s.NextToken(t)
		assert.Equal(t, c.Token, token)
	}
}

func TestNextTokenTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		token := s.NextToken(t)
		assert.Equal(t, c.Token, token)
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
		assert.Equal(t, testCalls[2*i].Token, tokens[0])
		assert.Equal(t, testCalls[2*i+1].Token, tokens[1])
	}
}

func TestNextTokensTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for i := 0; i < 5; i++ {
		tokens := s.NextTokens(t, 2)
		assert.Equal(t, testCalls[2*i].Token, tokens[0])
		assert.Equal(t, testCalls[2*i+1].Token, tokens[1])
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
		require.Equal(t, c.Token, token)
		require.Equal(t, c.Attrs, attributes)
	}
}

func TestNextCallTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		token, attributes := s.NextCall(t)
		require.Equal(t, c.Token, token)
		require.Equal(t, c.Attrs, attributes)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.NextCall(tt)
	assert.True(t, tt.Failed())
}

func TestExpectToken(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		s.ExpectToken(t, c.Token)
	}
}

func TestExpectTokenTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		s.ExpectToken(t, c.Token)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectToken(tt, []byte("foo"))
	assert.True(t, tt.Failed())
}

func TestExpectTokens(t *testing.T) {
	s, testCalls := sinkTest(t)
	for i := 0; i < 5; i++ {
		s.ExpectTokens(t, testCalls[2*i].Token, testCalls[2*i+1].Token)
	}
}

func TestExpectTokensTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for i := 0; i < 5; i++ {
		s.ExpectTokens(t, testCalls[2*i].Token, testCalls[2*i+1].Token)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectTokens(tt, []byte("foo"), []byte("bar"))
	assert.True(t, tt.Failed())
}

func TestExpectCall(t *testing.T) {
	s, testCalls := sinkTest(t)
	for _, c := range testCalls {
		s.ExpectCall(t, c.Token, c.Attrs)
	}
}

func TestExpectCallTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	for _, c := range testCalls {
		s.ExpectCall(t, c.Token, c.Attrs)
	}

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectCall(tt, []byte("foo"), nil)
	assert.True(t, tt.Failed())
}

func TestExpectCalls(t *testing.T) {
	s, testCalls := sinkTest(t)
	testCallsOutOfOrder := make([]*Call, 0, 10)
	for i := 0; i < len(testCalls); i += 2 {
		testCallsOutOfOrder = append(testCallsOutOfOrder, testCalls[i])
	}
	for i := 1; i < len(testCalls); i += 2 {
		testCallsOutOfOrder = append(testCallsOutOfOrder, testCalls[i])
	}
	s.ExpectCalls(t, testCallsOutOfOrder...)
}

func TestExpectCallsTimeout(t *testing.T) {
	s, testCalls := sinkTest(t, WithTimeout(10*time.Millisecond))
	testCallsOutOfOrder := make([]*Call, 0, 10)
	for i := 0; i < len(testCalls); i += 2 {
		testCallsOutOfOrder = append(testCallsOutOfOrder, testCalls[i])
	}
	for i := 1; i < len(testCalls); i += 2 {
		testCallsOutOfOrder = append(testCallsOutOfOrder, testCalls[i])
	}
	s.ExpectCalls(t, testCallsOutOfOrder...)

	// Create a new T so we can expect it to fail without failing the overall test.
	tt := new(testing.T)
	s.ExpectCalls(tt, new(Call))
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
		s.ExpectCall(t, testCalls[i].Token, testCalls[i].Attrs)
	}
}

func sinkTest(t *testing.T, opts ...SinkOpt) (*Sink, []*Call) {
	s := NewSink(opts...)
	testCalls := make([]*Call, 0, 10)
	for i := 0; i < 10; i++ {
		testCalls = append(testCalls, &Call{
			Token: []byte(fmt.Sprintf("token-%d", i)),
			Attrs: map[string]any{
				"key": fmt.Sprintf("value-%d", i),
			},
		})
	}
	go func() {
		for _, c := range testCalls {
			assert.NoError(t, s.Callback(context.Background(), []emit.Token{emit.NewToken(c.Token, c.Attrs)}))
		}
	}()
	return s, testCalls
}
