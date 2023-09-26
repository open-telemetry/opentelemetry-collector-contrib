// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splittest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Step struct {
	validate validate
}

type validate func(t *testing.T, advance int, token []byte, err error)

func ExpectToken(expectToken string) Step {
	return ExpectAdvanceToken(len(expectToken), expectToken)
}

func ExpectAdvanceToken(expectAdvance int, expectToken string) Step {
	return Step{
		validate: func(t *testing.T, advance int, token []byte, err error) {
			assert.Equal(t, expectAdvance, advance)
			assert.Equal(t, []byte(expectToken), token)
			assert.NoError(t, err)
		},
	}
}

func ExpectAdvanceNil(expectAdvance int) Step {
	return Step{
		validate: func(t *testing.T, advance int, token []byte, err error) {
			assert.Equal(t, expectAdvance, advance)
			assert.Equal(t, []byte(nil), token)
			assert.NoError(t, err)
		},
	}
}

func ExpectError(expectErr string) Step {
	return Step{
		validate: func(t *testing.T, advance int, token []byte, err error) {
			assert.EqualError(t, err, expectErr)
		},
	}
}

func New(splitFunc bufio.SplitFunc, input []byte, steps ...Step) func(*testing.T) {
	return func(t *testing.T) {
		var offset int
		var atEOF bool
		for _, step := range append(steps, expectMoreRequestAtEOF()) {
			// Split funcs do not have control over the size of the
			// buffer so must be able to ask for more data as needed.
			// Start with a tiny buffer and grow it slowly to ensure
			// the split func is capable of asking appropriately.
			var bufferSize int
			var advance int
			var token []byte
			var err error
			for !atEOF && needMoreData(advance, token, err) {
				// Grow the buffer at a slow pace to ensure that we're
				// exercising the split func's ability to ask for more data.
				bufferSize = 1 + bufferSize + bufferSize/8
				data := make([]byte, 0, bufferSize)
				if offset+bufferSize >= len(input) {
					atEOF = true
					data = append(data, input[offset:]...)
				} else {
					data = append(data, input[offset:offset+bufferSize]...)
				}
				advance, token, err = splitFunc(data, atEOF)
			}
			offset += advance
			step.validate(t, advance, token, err)
		}
	}
}

func expectMoreRequestAtEOF() Step {
	return Step{
		validate: func(t *testing.T, advance int, token []byte, err error) {
			assert.True(t, needMoreData(advance, token, err))
		},
	}
}

func needMoreData(advance int, token []byte, err error) bool {
	return advance == 0 && token == nil && err == nil
}
