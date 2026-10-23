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
var _ Server = (*udsServer)(nil)

// NewUDSServer creates a transport.Server using Unixgram as its transport.
func NewUDSServer(transport Transport, socketPath string, socketPermissions os.FileMode, socketBufferSize int) (Server, error) {
	if !transport.IsPacketTransport() {
		return nil, fmt.Errorf("NewUDSServer with %s: %w", transport.String(), ErrUnsupportedPacketTransport)
	}

	// Remove any stale socket file from a previous instance that didn't
	// shut down cleanly (e.g., crash, SIGKILL). Only remove if the file
	// is actually a socket to avoid accidentally deleting unrelated files.
	if fi, statErr := os.Stat(socketPath); statErr == nil && fi.Mode()&os.ModeSocket != 0 {
		_ = os.Remove(socketPath)
	}

	conn, err := net.ListenPacket(transport.String(), socketPath)
	if err != nil {
		return nil, fmt.Errorf("starting to listen %s socket: %w", transport.String(), err)
	}

	if socketBufferSize > 0 {
		if uc, ok := conn.(*net.UnixConn); ok {
			if err := uc.SetReadBuffer(socketBufferSize); err != nil {
				conn.Close()
				return nil, fmt.Errorf("setting socket buffer size: %w", err)
			}
		}
	}

	if err := os.Chmod(socketPath, socketPermissions); err != nil {
		return nil, fmt.Errorf("running chmod %v: %w", socketPermissions, err)
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
