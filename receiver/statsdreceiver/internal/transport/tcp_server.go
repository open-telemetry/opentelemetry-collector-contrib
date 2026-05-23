// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/consumer"
)

var errTCPServerDone = errors.New("server stopped")

type tcpServer struct {
	listener  net.Listener
	wg        sync.WaitGroup
	transport Transport
}

// Ensure that Server is implemented on TCP Server.
var _ Server = (*tcpServer)(nil)

// NewTCPServer creates a transport.Server using TCP as its transport.
func NewTCPServer(transport Transport, address string) (Server, error) {
	var tsrv tcpServer
	var err error

	if !transport.IsStreamTransport() {
		return nil, fmt.Errorf("NewTCPServer with %s: %w", transport.String(), ErrUnsupportedStreamTransport)
	}

	tsrv.transport = transport
	tsrv.listener, err = net.Listen(transport.String(), address)
	if err != nil {
		return nil, fmt.Errorf("starting to listen %s socket: %w", transport.String(), err)
	}

	return &tsrv, nil
}

// ListenAndServe starts the server ready to receive metrics.
func (t *tcpServer) ListenAndServe(nextConsumer consumer.Metrics, reporter Reporter, transferChan chan<- Metric) error {
	if nextConsumer == nil || reporter == nil {
		return errNilListenAndServeParameters
	}

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return errTCPServerDone
			}
			reporter.OnDebugf("TCP Transport - Accept error: %v", err)
			continue
		}

		t.wg.Go(func() {
			handleTCPConn(conn, reporter, transferChan)
		})
	}
}

// handleTCPConn is helper that parses the buffer and split it line by line to be parsed upstream.
func handleTCPConn(c net.Conn, reporter Reporter, transferChan chan<- Metric) {
	payload := make([]byte, 4096)
	var remainder []byte
	for {
		n, err := c.Read(payload)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				reporter.OnDebugf("TCP transport (%s) Error reading payload: %v", c.LocalAddr(), err)
			}
			return
		}
		buf := bytes.NewBuffer(append(remainder, payload[0:n]...))
		for {
			bytes, err := buf.ReadBytes(byte('\n'))
			if errors.Is(err, io.EOF) {
				if len(bytes) != 0 {
					remainder = bytes
				}
				break
			}
			line := strings.TrimSpace(string(bytes))
			if line != "" {
				transferChan <- Metric{line, c.RemoteAddr()}
			}
		}
	}
}

// Close closes the server.
func (t *tcpServer) Close() error {
	err := t.listener.Close()
	t.wg.Wait()
	return err
}
