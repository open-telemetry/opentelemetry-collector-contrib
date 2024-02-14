// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splittest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"

import (
	"bufio"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type Step struct {
	tick     time.Duration
	timeout  time.Duration
	validate func(t *testing.T, advance int, token []byte, err error)
}

func ExpectReadMore() Step {
	return Step{
		validate: func(t *testing.T, advance int, token []byte, err error) {
			assert.True(t, needMoreData(advance, token, err))
		},
	}
}

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

func Eventually(step Step, maxTime time.Duration, tick time.Duration) Step {
	step.tick = tick
	step.timeout = maxTime
	return step
}

func New(splitFunc bufio.SplitFunc, input []byte, steps ...Step) func(*testing.T) {
	return func(t *testing.T) {
		var offset int
		for _, step := range append(steps, ExpectReadMore()) {
			// Split funcs do not have control over the size of the
			// buffer so must be able to ask for more data as needed.
			// Start with a tiny buffer and grow it slowly to ensure
			// the split func is capable of asking appropriately.
			var bufferSize int
			var atEOF bool
			var advance int
			var token []byte
			var err error

			var waited time.Duration
			for needMoreData(advance, token, err) {
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
				// t.Errorf("\nbuffer: %d, advance: %d, token: %q, err: %v", bufferSize, advance, token, err)

				if atEOF {
					if waited >= step.timeout {
						break
					}
					time.Sleep(step.tick)
					waited += step.tick
				}
			}
			offset += advance
			step.validate(t, advance, token, err)
		}
	}
}

func needMoreData(advance int, token []byte, err error) bool {
	return advance == 0 && token == nil && err == nil
}

// ScanLinesStrict behaves like bufio.ScanLines except EOF is not considered a line ending.
func ScanLinesStrict(data []byte, atEOF bool) (advance int, token []byte, err error) {
	advance, token, err = bufio.ScanLines(data, atEOF)
	if advance == len(token) {
		return 0, nil, nil
	}
	return
}

func GenerateBytes(length int) []byte {
	chars := []byte(`abcdefghijklmnopqrstuvwxyz`)
	newSlice := make([]byte, length)
	for i := 0; i < length; i++ {
		newSlice[i] = chars[i%len(chars)]
	}
	return newSlice
}
