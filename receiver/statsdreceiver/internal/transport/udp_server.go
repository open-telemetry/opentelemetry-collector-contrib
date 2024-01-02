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

	"go.opentelemetry.io/collector/consumer"
)

type udpServer struct {
	packetConn net.PacketConn
	transport  Transport
}

// Ensure that Server is implemented on UDP Server.
var _ (Server) = (*udpServer)(nil)

// NewUDPServer creates a transport.Server using UDP as its transport.
func NewUDPServer(transport Transport, address string) (Server, error) {
	if !transport.IsPacketTransport() {
		return nil, fmt.Errorf("NewUDPServer with %s: %w", transport.String(), ErrUnsupportedPacketTransport)
	}

	conn, err := net.ListenPacket(transport.String(), address)
	if err != nil {
		return nil, fmt.Errorf("starting to listen %s socket: %w", transport.String(), err)
	}

	return &udpServer{
		packetConn: conn,
		transport:  transport,
	}, nil
}

// ListenAndServe starts the server ready to receive metrics.
func (u *udpServer) ListenAndServe(
	nextConsumer consumer.Metrics,
	reporter Reporter,
	transferChan chan<- Metric,
) error {
	if nextConsumer == nil || reporter == nil {
		return errNilListenAndServeParameters
	}

	buf := make([]byte, 65527) // max size for udp packet body (assuming ipv6)
	for {
		n, addr, err := u.packetConn.ReadFrom(buf)
		if n > 0 {
			bufCopy := make([]byte, n)
			copy(bufCopy, buf)
			u.handlePacket(bufCopy, addr, transferChan)
		}
		if err != nil {
			reporter.OnDebugf("%s Transport (%s) - ReadFrom error: %v",
				u.transport,
				u.packetConn.LocalAddr(),
				err)
			var netErr net.Error
			if errors.As(err, &netErr) {
				if netErr.Timeout() {
					continue
				}
			}
			return err
		}
	}
}

// Close closes the server.
func (u *udpServer) Close() error {
	return u.packetConn.Close()
}

// handlePacket is helper that parses the buffer and split it line by line to be parsed upstream.
func (u *udpServer) handlePacket(
	data []byte,
	addr net.Addr,
	transferChan chan<- Metric,
) {
	buf := bytes.NewBuffer(data)
	for {
		bytes, err := buf.ReadBytes((byte)('\n'))
		if errors.Is(err, io.EOF) {
			if len(bytes) == 0 {
				// Completed without errors.
				break
			}
		}
		line := strings.TrimSpace(string(bytes))
		if line != "" {
			transferChan <- Metric{line, addr}
		}
	}
}
