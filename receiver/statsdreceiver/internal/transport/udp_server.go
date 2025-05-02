// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import (
	"fmt"
	"net"
)

type udpServer struct {
	packetServer
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
		packetServer: packetServer{
			packetConn: conn,
			transport:  transport,
		},
	}, nil
}

// Close closes the server.
func (u *udpServer) Close() error {
	return u.packetConn.Close()
}
