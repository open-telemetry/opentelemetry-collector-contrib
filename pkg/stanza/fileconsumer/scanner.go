// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"errors"
	"io"

	stanzaerrors "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

// PositionalScanner is a scanner that maintains position
//
// Deprecated: [v0.80.0] This will be made internal in a future release, tentatively v0.82.0.
type PositionalScanner struct {
	pos int64
	*bufio.Scanner
}

// NewPositionalScanner creates a new positional scanner
//
// Deprecated: [v0.80.0] This will be made internal in a future release, tentatively v0.82.0.
func NewPositionalScanner(r io.Reader, maxLogSize int, bufferCap int, startOffset int64, splitFunc bufio.SplitFunc) *PositionalScanner {
	ps := &PositionalScanner{
		pos:     startOffset,
		Scanner: bufio.NewScanner(r),
	}

	buf := make([]byte, 0, bufferCap)
	ps.Scanner.Buffer(buf, maxLogSize*2)

	scanFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)
		if (advance == 0 && token == nil && err == nil) && len(data) >= maxLogSize {
			// reference: https://pkg.go.dev/bufio#SplitFunc
			// splitFunc returns (0, nil, nil) to signal the Scanner to read more data but the buffer is full.
			// Truncate the log entry.
			advance, token, err = maxLogSize, data[:maxLogSize], nil
		} else if len(token) > maxLogSize {
			advance, token = maxLogSize, token[:maxLogSize]
		}
		ps.pos += int64(advance)
		return
	}
	ps.Scanner.Split(scanFunc)
	return ps
}

// Pos returns the current position of the scanner
func (ps *PositionalScanner) Pos() int64 {
	return ps.pos
}

func (ps *PositionalScanner) getError() error {
	err := ps.Err()
	if errors.Is(err, bufio.ErrTooLong) {
		return stanzaerrors.NewError("log entry too large", "increase max_log_size or ensure that multiline regex patterns terminate")
	} else if err != nil {
		return stanzaerrors.Wrap(err, "scanner error")
	}
	return nil
}
