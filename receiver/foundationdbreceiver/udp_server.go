// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"net" // UDP Server Section
)

type fdbTraceListener interface {
	ListenAndServe(handler fdbTraceHandler, maxPacketSize int) error
	Close() error
}

type udpServer struct {
	conn *net.UDPConn
}

// NewUDPServer creates a transport.Server using UDP as its transport.
func NewUDPServer(addrString string, sockerBufferSize int) (*udpServer, error) {
	addr, err := net.ResolveUDPAddr("udp", addrString)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(addr.Network(), addr)
	if err != nil {
		return nil, err
	}

	if sockerBufferSize > 0 {
		err := conn.SetReadBuffer(sockerBufferSize)
		if err != nil {
			return nil, err
		}
	}

	return &udpServer{
		conn: conn,
	}, nil
}

func (u *udpServer) ListenAndServe(handler fdbTraceHandler, maxPacketSize int) error {
	buf := make([]byte, maxPacketSize) // max size for udp packet body (assuming ipv6)
	for {
		n, _, err := u.conn.ReadFrom(buf)
		if n > 0 {
			bufCopy := make([]byte, n)
			copy(bufCopy, buf)
			processingErr := handler.Handle(bufCopy)
			if processingErr != nil {
				return processingErr
			}
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok {
				if netErr.Timeout() {
					continue
				}
			}
			return err
		}
	}
}

func (u *udpServer) Close() error {
	return u.conn.Close()
}
