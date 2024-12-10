// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import (
	"fmt"
	"net"
	"os"
)

type udsServer struct {
	packetServer
}

// Ensure that Server is implemented on UDS Server.
var _ (Server) = (*udsServer)(nil)

// NewUDSServer creates a transport.Server using Unixgram as its transport.
func NewUDSServer(transport Transport, socketPath string) (Server, error) {
	if !transport.IsPacketTransport() {
		return nil, fmt.Errorf("NewUDSServer with %s: %w", transport.String(), ErrUnsupportedPacketTransport)
	}

	if err := prepareSocket(socketPath); err != nil {
		return nil, err
	}

	conn, err := net.ListenPacket(transport.String(), socketPath)
	if err != nil {
		return nil, fmt.Errorf("starting to listen %s socket: %w", transport.String(), err)
	}

	return &udsServer{
		packetServer: packetServer{
			packetConn: conn,
			transport:  transport,
		},
	}, nil
}

// Close closes the server.
func (u *udsServer) Close() error {
	os.Remove(u.packetConn.LocalAddr().String())
	return u.packetConn.Close()
}

func prepareSocket(socketPath string) error {
	if _, err := os.Stat(socketPath); err == nil {
		// File exists, remove it
		if err = os.Remove(socketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// Return any error that's not "file does not exist"
		return fmt.Errorf("failed to stat socket file: %w", err)
	}

	return nil
}
