// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splittest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"

import (
	"bufio"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// state is going to keep processing state of the testReader
type state struct {
	ReadFrom  int
	Processed int
}

// testReader is a testReader which keeps state of readed and processed data
type testReader struct {
	State *state
	Data  []byte
}

// newTestReader creates testReader with empty state
func newTestReader(data []byte) testReader {
	return testReader{
		State: &state{
			ReadFrom:  0,
			Processed: 0,
		},
		Data: data,
	}
}

// Read reads data from testReader and remebers where reading has been finished
func (r testReader) Read(p []byte) (n int, err error) {
	// return eof if data has been fully readed
	if len(r.Data)-r.State.ReadFrom == 0 {
		return 0, io.EOF
	}

	// iterate over data char by char and write into p
	// until p is full or no more data left to read
	i := 0
	for ; i < len(r.Data)-r.State.ReadFrom; i++ {
		if i == len(p) {
			break
		}
		p[i] = r.Data[r.State.ReadFrom+i]
	}

	// update state
	r.State.ReadFrom += i
	return i, nil
}

// Reset resets testReader state (sets last readed position to last processed position)
func (r *testReader) Reset() {
	r.State.ReadFrom = r.State.Processed
}

func (r *testReader) splitFunc(split bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = split(data, atEOF)
		r.State.Processed += advance
		return
	}
}

type TestCase struct {
	Name                        string
	Pattern                     string
	Input                       []byte
	ExpectedTokens              []string
	ExpectedError               error
	Sleep                       time.Duration
	AdditionalIterations        int
	PreserveLeadingWhitespaces  bool
	PreserveTrailingWhitespaces bool
}

func (tc TestCase) Run(split bufio.SplitFunc) func(t *testing.T) {
	reader := newTestReader(tc.Input)

	return func(t *testing.T) {
		var tokens []string
		for i := 0; i < 1+tc.AdditionalIterations; i++ {
			// sleep before next iterations
			if i > 0 {
				time.Sleep(tc.Sleep)
			}
			reader.Reset()
			scanner := bufio.NewScanner(reader)
			scanner.Split(reader.splitFunc(split))
			for {
				ok := scanner.Scan()
				if !ok {
					assert.Equal(t, tc.ExpectedError, scanner.Err())
					break
				}
				tokens = append(tokens, scanner.Text())
			}
		}

		assert.Equal(t, tc.ExpectedTokens, tokens)
	}
}

func GenerateBytes(length int) []byte {
	chars := []byte(`abcdefghijklmnopqrstuvwxyz`)
	newSlice := make([]byte, length)
	for i := 0; i < length; i++ {
		newSlice[i] = chars[i%len(chars)]
	}
	return newSlice
}
