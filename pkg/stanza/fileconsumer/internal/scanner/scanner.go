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
	pos int64
	*bufio.Scanner
}

// New creates a new positional scanner
func New(r io.Reader, maxLogSize int, bufferSize int, startOffset int64, splitFunc bufio.SplitFunc) *Scanner {
	s := &Scanner{Scanner: bufio.NewScanner(r), pos: startOffset}
	s.Buffer(make([]byte, 0, bufferSize), maxLogSize)
	scanFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)
		s.pos += int64(advance)
		return
	}
	s.Split(scanFunc)
	return s
}

// Pos returns the current position of the scanner
func (s *Scanner) Pos() int64 {
	return s.pos
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
