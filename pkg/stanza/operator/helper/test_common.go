// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"bufio"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// state is going to keep processing state of the TestReader
type state struct {
	ReadFrom  int
	Processed int
}

// TestReader is a TestReader which keeps state of readed and processed data
type TestReader struct {
	State *state
	Data  []byte
}

// NewReader creates TestReader with empty state
func NewReader(data []byte) TestReader {
	return TestReader{
		State: &state{
			ReadFrom:  0,
			Processed: 0,
		},
		Data: data,
	}
}

// Read reads data from TestReader and remebers where reading has been finished
func (r TestReader) Read(p []byte) (n int, err error) {
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

// Reset resets TestReader state (sets last readed position to last processed position)
func (r *TestReader) Reset() {
	r.State.ReadFrom = r.State.Processed
}

func (r *TestReader) SplitFunc(splitFunc bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)
		r.State.Processed += advance
		return
	}
}

type TokenizerTestCase struct {
	Name                        string
	Pattern                     string
	Raw                         []byte
	ExpectedTokenized           []string
	ExpectedError               error
	Flusher                     *Flusher
	Sleep                       time.Duration
	AdditionalIterations        int
	PreserveLeadingWhitespaces  bool
	PreserveTrailingWhitespaces bool
}

func (tc TokenizerTestCase) RunFunc(splitFunc bufio.SplitFunc) func(t *testing.T) {
	reader := NewReader(tc.Raw)

	return func(t *testing.T) {
		var tokenized []string
		for i := 0; i < 1+tc.AdditionalIterations; i++ {
			// sleep before next iterations
			if i > 0 {
				time.Sleep(tc.Sleep)
			}
			reader.Reset()
			scanner := bufio.NewScanner(reader)
			scanner.Split(reader.SplitFunc(splitFunc))
			for {
				ok := scanner.Scan()
				if !ok {
					assert.Equal(t, tc.ExpectedError, scanner.Err())
					break
				}
				tokenized = append(tokenized, scanner.Text())
			}
		}

		assert.Equal(t, tc.ExpectedTokenized, tokenized)
	}
}

func GeneratedByteSliceOfLength(length int) []byte {
	chars := []byte(`abcdefghijklmnopqrstuvwxyz`)
	newSlice := make([]byte, length)
	for i := 0; i < length; i++ {
		newSlice[i] = chars[i%len(chars)]
	}
	return newSlice
}
