// Copyright 2022 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
				if netErr.Temporary() {
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
