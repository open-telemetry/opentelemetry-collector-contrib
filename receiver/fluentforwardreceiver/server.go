// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/tinylib/msgp/msgp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

// The initial size of the read buffer. Messages can come in that are bigger
// than this, but this serves as a starting point.
const readBufferSize = 10 * 1024

type server struct {
	outCh            chan<- eventWithACK
	ackWaitTimeout   time.Duration
	ackWaiters       chan struct{}
	logger           *zap.Logger
	telemetryBuilder *metadata.TelemetryBuilder
	conns            map[net.Conn]struct{}
	mu               sync.Mutex
}

func newServer(outCh chan<- eventWithACK, ackWaitTimeout time.Duration, logger *zap.Logger, telemetryBuilder *metadata.TelemetryBuilder) *server {
	return &server{
		outCh:            outCh,
		ackWaitTimeout:   ackWaitTimeout,
		ackWaiters:       make(chan struct{}, maxPendingACKs),
		logger:           logger,
		telemetryBuilder: telemetryBuilder,
		conns:            make(map[net.Conn]struct{}),
	}
}

func (s *server) Start(ctx context.Context, listener net.Listener) {
	go func() {
		s.handleConnections(ctx, listener)
		if ctx.Err() == nil {
			panic("logic error in receiver, connections should always be listened for while receiver is running")
		}
	}()
}

func (s *server) handleConnections(ctx context.Context, listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if ctx.Err() != nil {
			return
		}
		// If there is an error and the receiver isn't shutdown, we need to
		// keep trying to accept connections if at all possible. Put in a sleep
		// to prevent hot loops in case the error persists.
		if err != nil {
			timer := time.NewTimer(10 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				continue
			}
		}

		s.telemetryBuilder.FluentOpenedConnections.Add(ctx, 1)

		s.logger.Debug("Got connection", zap.String("remoteAddr", conn.RemoteAddr().String()))
		s.addConn(conn)

		go func() {
			defer s.telemetryBuilder.FluentClosedConnections.Add(ctx, 1)
			defer s.removeConn(conn)

			err := s.handleConn(ctx, conn)
			if err != nil {
				if errors.Is(err, io.EOF) {
					s.logger.Debug("Closing connection", zap.String("remoteAddr", conn.RemoteAddr().String()), zap.Error(err))
				} else {
					s.logger.Debug("Unexpected error handling connection", zap.String("remoteAddr", conn.RemoteAddr().String()), zap.Error(err))
				}
			}
			conn.Close()
		}()
	}
}

func (s *server) handleConn(ctx context.Context, conn net.Conn) error {
	reader := msgp.NewReaderSize(conn, readBufferSize)

	for {
		mode, err := determineNextEventMode(reader.R)
		if err != nil {
			return err
		}

		var e event
		switch mode {
		case unknownMode:
			return errors.New("could not determine e mode")
		case messageMode:
			e = &messageEventLogRecord{}
		case forwardMode:
			e = &forwardEventLogRecords{}
		case packedForwardMode:
			e = &packedForwardEventLogRecords{}
		default:
			panic("programmer bug in mode handling")
		}

		err = e.DecodeMsg(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.telemetryBuilder.FluentParseFailures.Add(ctx, 1)
			}
			return fmt.Errorf("failed to parse %s mode e: %w", mode.String(), err)
		}

		s.telemetryBuilder.FluentEventsParsed.Add(ctx, 1)

		chunk := e.Chunk()
		if chunk == "" {
			if err = s.enqueueEvent(ctx, eventWithACK{event: e}, 0); err != nil {
				return err
			}
			continue
		}

		if !s.acquireACKWaiter(ctx) {
			return fmt.Errorf("too many pending acknowledgments")
		}
		ackCh := make(chan error, 1)
		err = s.enqueueEvent(ctx, eventWithACK{event: e, ackCh: ackCh}, s.ackWaitTimeout)
		if err != nil {
			s.releaseACKWaiter()
			return err
		}

		err = s.waitForACK(ctx, conn, chunk, ackCh)
		s.releaseACKWaiter()
		if err != nil {
			return err
		}
	}
}

func (s *server) enqueueEvent(ctx context.Context, e eventWithACK, timeout time.Duration) error {
	if timeout <= 0 {
		select {
		case s.outCh <- e:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case s.outCh <- e:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timed out waiting to enqueue chunk for acknowledgment")
	}
}

func (s *server) waitForACK(ctx context.Context, conn net.Conn, chunk string, ackCh <-chan error) error {
	timer := time.NewTimer(s.ackWaitTimeout)
	defer timer.Stop()

	select {
	case err := <-ackCh:
		if err != nil {
			return fmt.Errorf("downstream consumer failed processing chunk %s: %w", chunk, err)
		}
		err = msgp.Encode(conn, internal.AckResponse{Ack: chunk})
		if err != nil {
			return fmt.Errorf("failed to acknowledge chunk %s: %w", chunk, err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timed out waiting for downstream consumer to process chunk %s", chunk)
	}
}

func (s *server) acquireACKWaiter(ctx context.Context) bool {
	select {
	case s.ackWaiters <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

func (s *server) releaseACKWaiter() {
	select {
	case <-s.ackWaiters:
	default:
	}
}

// determineNextEventMode inspects the next bit of data from the given peeker
// reader to determine which type of event mode it is.  According to the
// forward protocol spec: "Server MUST detect the carrier mode by inspecting
// the second element of the array."  It is assumed that peeker is aligned at
// the start of a new event, otherwise the result is undefined and will
// probably error.
func determineNextEventMode(peeker peeker) (eventMode, error) {
	var chunk []byte
	var err error
	chunk, err = peeker.Peek(2)
	if err != nil {
		return unknownMode, err
	}

	// The first byte is the array header, which will always be 1 byte since no
	// message modes have more than 4 entries. So skip to the second byte which
	// is the tag string header.
	tagType := chunk[1]
	// We already read the first type for the type
	tagLen := 1

	isFixStr := tagType&0b10100000 == 0b10100000
	if isFixStr {
		tagLen += int(tagType & 0b00011111)
	} else {
		switch tagType {
		case 0xd9:
			chunk, err = peeker.Peek(3)
			if err != nil {
				return unknownMode, err
			}
			tagLen += 1 + int(chunk[2])
		case 0xda:
			chunk, err = peeker.Peek(4)
			if err != nil {
				return unknownMode, err
			}
			tagLen += 2 + int(binary.BigEndian.Uint16(chunk[2:]))
		case 0xdb:
			chunk, err = peeker.Peek(6)
			if err != nil {
				return unknownMode, err
			}
			tagLen += 4 + int(binary.BigEndian.Uint32(chunk[2:]))
		default:
			return unknownMode, errors.New("malformed tag field")
		}
	}

	// Skip past the first byte (array header) and the entire tag and then get
	// one byte into the second field -- that is enough to know its type.
	chunk, err = peeker.Peek(1 + tagLen + 1)
	if err != nil {
		return unknownMode, err
	}

	secondElmType := msgp.NextType(chunk[1+tagLen:])

	switch secondElmType {
	case msgp.IntType, msgp.UintType, msgp.ExtensionType:
		return messageMode, nil
	case msgp.ArrayType:
		return forwardMode, nil
	case msgp.BinType, msgp.StrType:
		return packedForwardMode, nil
	default:
		return unknownMode, fmt.Errorf("unable to determine next event mode for type %v", secondElmType)
	}
}

func (s *server) addConn(c net.Conn) {
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
}

func (s *server) removeConn(c net.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

func (s *server) closeAllConns() {
	s.mu.Lock()
	for c := range s.conns {
		_ = c.Close() // Ignore errors
	}
	s.mu.Unlock()
}
