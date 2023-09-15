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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/protocol"
)

var errTCPServerDone = errors.New("server stopped")

type tcpServer struct {
	listener net.Listener
	reporter Reporter
	wg       sync.WaitGroup
	stopChan chan struct{}
}

var _ Server = (*tcpServer)(nil)

// NewTCPServer creates a transport.Server using TCP as its transport.
func NewTCPServer(addr string) (Server, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	t := tcpServer{
		listener: l,
		stopChan: make(chan struct{}),
	}
	return &t, nil
}

func (t *tcpServer) ListenAndServe(parser protocol.Parser, nextConsumer consumer.Metrics, reporter Reporter, transferChan chan<- Metric) error {
	if parser == nil || nextConsumer == nil || reporter == nil {
		return errNilListenAndServeParameters
	}

	t.reporter = reporter
LOOP:
	for {
		connChan := make(chan net.Conn, 1)
		go func() {
			c, err := t.listener.Accept()
			if err != nil {
				t.reporter.OnDebugf("TCP Transport - Accept error: %v",
					err)
			} else {
				connChan <- c
			}
		}()

		select {
		case conn := <-connChan:
			t.wg.Add(1)
			go t.handleConn(conn, transferChan)
		case <-t.stopChan:
			break LOOP
		}
	}
	return errTCPServerDone
}

func (t *tcpServer) handleConn(c net.Conn, transferChan chan<- Metric) {
	payload := make([]byte, 4096)
	var remainder []byte
	for {
		n, err := c.Read(payload)
		if err != nil {
			t.reporter.OnDebugf("TCP transport (%s) Error reading payload: %v", c.LocalAddr(), err)
			t.wg.Done()
			return
		}
		buf := bytes.NewBuffer(append(remainder, payload[0:n]...))
		for {
			bytes, err := buf.ReadBytes((byte)('\n'))
			if errors.Is(err, io.EOF) {
				if len(bytes) != 0 {
					remainder = bytes
				}
				break
			}
			line := strings.TrimSpace(string(bytes))
			if line != "" {
				transferChan <- Metric{line, c.LocalAddr()}
			}
		}
	}
}

func (t *tcpServer) Close() error {
	fmt.Println("FOFOODFD")
	close(t.stopChan)
	t.wg.Wait()
	return t.listener.Close()
}
