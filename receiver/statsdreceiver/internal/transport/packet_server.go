// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import (
	"errors"
	"net"

	"go.opentelemetry.io/collector/consumer"
)

type packetServer struct {
	packetConn net.PacketConn
	transport  Transport
}

// ListenAndServe starts the server ready to receive metrics.
func (u *packetServer) ListenAndServe(
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
		if addr == nil && u.transport == UDS {
			addr = &udsAddr{
				network: u.transport.String(),
				address: u.packetConn.LocalAddr().String(),
			}
		}

		if n > 0 {
			u.handlePacket(n, buf, addr, transferChan)
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

// handlePacket is helper that parses the buffer and split it line by line to be parsed upstream.
func (u *packetServer) handlePacket(
	numBytes int,
	data []byte,
	addr net.Addr,
	transferChan chan<- Metric,
) {
	splitPacket := NewSplitBytes(data[:numBytes], '\n')
	for splitPacket.Next() {
		chunk := splitPacket.Chunk()
		if len(chunk) > 0 {
			transferChan <- Metric{string(chunk), addr}
		}
	}
}

type udsAddr struct {
	network string
	address string
}

func (u *udsAddr) Network() string {
	return u.network
}

func (u *udsAddr) String() string {
	return u.address
}
