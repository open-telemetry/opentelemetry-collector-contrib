// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scanner // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"

import (
	"bufio"
	"errors"
	"io"

	stanzaerrors "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

const DefaultBufferSize = 16 * 1024

// Scanner is a scanner that maintains position
type Scanner struct {
	offsets []int64
	*bufio.Scanner
}

// New creates a new positional scanner
func New(r io.Reader, maxLogSize int, buf []byte, startOffset int64, splitFunc bufio.SplitFunc) *Scanner {
	s := &Scanner{Scanner: bufio.NewScanner(r), offsets: []int64{startOffset}}
	s.Buffer(buf, maxLogSize)
	scanFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)
		if advance > 0 {
			s.offsets = append(s.offsets, s.offsets[len(s.offsets)-1]+int64(advance))
		}
		return
	}
	s.Split(scanFunc)
	return s
}

func (s *Scanner) Offsets() []int64 {
	return s.offsets
}

// Pos returns the current position of the scanner
func (s *Scanner) Pos() int64 {
	return s.offsets[len(s.offsets)-1]
}

func (s *Scanner) ClearOffsets() {
	s.offsets = s.offsets[len(s.offsets)-1:]
}

func (s *Scanner) Error() error {
	err := s.Err()
	if errors.Is(err, bufio.ErrTooLong) {
		return stanzaerrors.NewError("log entry too large", "increase max_log_size or ensure that multiline regex patterns terminate")
	}
	if err != nil {
		return stanzaerrors.Wrap(err, "scanner error")
	}
	return nil
}
