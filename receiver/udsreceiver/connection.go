// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package udsreceiver

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

const (
	bufSize       = 65535 // size of our read buffer
	somLen        = 4     // length of start of message identifier
	minHeaderSize = 17    // minimum size of message header
)

var (
	som = []byte("\x00\x00\x00\x00")
)

// Handler interface allows for pluggable net.Conn compatible stream oriented
// network transports.
type Handler interface {
	Handle(net.Conn)
}

// ConnectionHandler implements our UDS Daemon protocol.
type ConnectionHandler struct {
	Logger    log.Logger
	Processor Processor
}

// Handle implements Handler and can use a generic net.Conn network connection
// to exchange data with UDs Daemon clients.
func (c *ConnectionHandler) Handle(conn net.Conn) {
	_ = level.Info(c.Logger).Log("msg", "connected")
	defer func() {
		_ = level.Info(c.Logger).Log("msg", "disconnected")
	}()

	hnd := connection{l: c.Logger, proc: c.Processor}
	hnd.handle(conn)
}

type connection struct {
	l       log.Logger
	pid     uint64
	tid     uint64
	float32 *bool
	msg     *Message
	proc    Processor
}

func (p *connection) handle(conn net.Conn) {
	var (
		buf    [bufSize]byte
		offset int
	)

	for {
		// read from unix socket
		n, err := conn.Read(buf[offset:])
		if err != nil {
			if err != io.EOF {
				_ = level.Error(p.l).Log(
					"pid", p.pid,
					"tid", p.tid,
					"msg", err.Error(),
				)
			}
			return
		}

		if offset > 0 && bytes.HasPrefix(buf[offset:], som) {
			// we just received (the beginning of) a new Message while having
			// existing unfinished Message data. We'll have to drop the existing
			// payload as it won't be finished.
			_ = level.Warn(p.l).Log(
				"pid", p.pid,
				"tid", p.tid,
				"msg", "received new payload, previous payload incomplete",
			)
			copy(buf[:], buf[offset:offset+n])
			offset = 0
		}

		offset += n

		if p.msg == nil && offset < minHeaderSize {
			// not enough data to satisfy smallest possible header
			continue
		}

		// try to parse messages
		var (
			start     int
			processed int
			done      bool
		)

		// consume all available and complete messages
		for !done {
			processed, done = p.parseMessage(buf[start:offset])
			start += processed
		}

		// test buffer remainder
		if start == offset {
			// consumed all data...
			offset = 0
			continue
		}
		if start > 0 {
			// purge processed data and move beginning of new message to front
			copy(buf[:], buf[start:offset])
			offset -= start
			start = 0
		}
	}
}

func (p *connection) parseMessage(buf []byte) (consumed int, done bool) {
	var idx, n int

	if p.msg == nil {
		// check if we have a lingering truncated Message payload at the start
		// of our buffer
		idx = bytes.Index(buf, som)
		switch idx {
		case -1:
			// start marker not found, remove up to potential beginning of the
			// next message's start marker
			minTestPos := len(buf) - somLen
			if minTestPos < 0 {
				minTestPos = len(buf)
			}
			for i := len(buf); i > minTestPos; i-- {
				if buf[i-1] != 0 {
					return i, true
				}
			}
			return minTestPos, true
		case 0:
			// at beginning of Message
		default:
			// we have lingering data
			_ = level.Warn(p.l).Log(
				"pid", p.pid,
				"tid", p.tid,
				"msg", "ignoring lingering data",
			)
			if len(buf)-idx < minHeaderSize {
				// not enough data available for a Message header, bail out.
				return idx, true
			}
		}

		// try to parse header
		p.msg, n = p.parseHeader(buf[idx:])
		switch {
		case n < 0:
			// decoding error, invalidate this Message
			_ = level.Error(p.l).Log(
				"pid", p.pid,
				"tid", p.tid,
				"msg", "error while decoding varint, invalidating Message",
			)
			return idx - n, false
		case n == 0:
			// ran out of buffer... wait for more data
			return 0, true
		default:
			// we have successfully read the header, advance our index
			idx += n
		}
	}

	// try to read Message payload
	remainder := p.msg.AppendData(buf[idx:])
	if remainder > 0 {
		// received partial payload.
		return len(buf), true
	}

	// received full payload
	p.msg.ReceiveTime = time.Now()
	p.proc.Process(p.msg)
	p.msg = nil

	return len(buf) + remainder, remainder == 0
}

func (p *connection) parseHeader(buf []byte) (hdr *Message, n int) {
	// advance beyond start of Message marker
	idx := somLen

	defer func() {
		if recover() != nil {
			// we ran out of buffer
			hdr = nil
			n = 0
		}
	}()

	hdr = &Message{Type: MessageType(buf[idx])}
	idx++

	hdr.SequenceNr, n = binary.Uvarint(buf[idx:])
	if n < 1 {
		return nil, n
	}
	idx += n

	hdr.ProcessID, n = binary.Uvarint(buf[idx:])
	if n < 1 {
		return nil, n
	}
	p.pid = hdr.ProcessID
	idx += n

	hdr.ThreadID, n = binary.Uvarint(buf[idx:])
	if n < 1 {
		return nil, n
	}
	p.tid = hdr.ThreadID
	idx += n

	startTime := buf[idx : idx+8]
	if p.float32 == nil {
		isFloat32 := bytes.Equal(startTime[0:2], []byte("\x00\x00")) &&
			bytes.Equal(startTime[6:8], []byte("\x00\x00"))
		p.float32 = &isFloat32
	}
	if *p.float32 {
		// 32 bit float
		f := float64(math.Float32frombits(binary.BigEndian.Uint32(startTime[2:6])))
		hdr.StartTime = time.Unix(int64(f), int64((f-math.Floor(f))*1e9))
	} else {
		// 64 bit float
		f := math.Float64frombits(binary.BigEndian.Uint64(startTime))
		hdr.StartTime = time.Unix(int64(f), int64((f-math.Floor(f))*1e9))
	}
	hdr.Float32 = *p.float32
	idx += 8

	hdr.MsgLen, n = binary.Uvarint(buf[idx:])
	if n < 1 {
		return nil, n
	}
	idx += n

	// create RawPayload storage capacity
	hdr.RawPayload = make([]byte, 0, hdr.MsgLen)

	return hdr, idx
}
