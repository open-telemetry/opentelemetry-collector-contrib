// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import "bytes"

// SplitBytes iterates over a byte buffer, returning chunks split by a given
// delimiter byte. It does not perform any allocations, and does not modify the
// buffer it is given. It is not safe for use by concurrent goroutines.
//
//	sb := NewSplitBytes(buf, '\n')
//	for sb.Next() {
//	    fmt.Printf("%q\n", sb.Chunk())
//	}
//
// The sequence of chunks returned by SplitBytes is equivalent to calling
// bytes.Split, except without allocating an intermediate slice.
type SplitBytes struct {
	buf          []byte
	delim        byte
	currentChunk []byte
	lastChunk    bool
}

// NewSplitBytes initializes a SplitBytes struct with the provided buffer and delimiter.
func NewSplitBytes(buf []byte, delim byte) *SplitBytes {
	return &SplitBytes{
		buf:   buf,
		delim: delim,
	}
}

// Next advances SplitBytes to the next chunk, returning true if a new chunk
// actually exists and false otherwise.
func (sb *SplitBytes) Next() bool {
	if sb.lastChunk {
		// we do not check the length here, this ensures that we return the
		// last chunk in the sequence (even if it's empty)
		return false
	}

	next := bytes.IndexByte(sb.buf, sb.delim)
	if next == -1 {
		// no newline, consume the entire buffer
		sb.currentChunk = sb.buf
		sb.buf = nil
		sb.lastChunk = true
	} else {
		sb.currentChunk = sb.buf[:next]
		sb.buf = sb.buf[next+1:]
	}
	return true
}

// Chunk returns the current chunk.
func (sb *SplitBytes) Chunk() []byte {
	return sb.currentChunk
}
