// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"bytes"
	"io"
	"net"
	"strings"

	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

type udpServer struct {
	packetConn net.PacketConn
	reporter   Reporter
}

var _ (Server) = (*udpServer)(nil)

// NewUDPServer creates a transport.Server using UDP as its transport.
func NewUDPServer(addr string) (Server, error) {
	packetConn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	u := udpServer{
		packetConn: packetConn,
	}
	return &u, nil
}

func (u *udpServer) ListenAndServe(
	parser protocol.Parser,
	nextConsumer consumer.Metrics,
	reporter Reporter,
	transferChan chan<- string,
) error {
	if parser == nil || nextConsumer == nil || reporter == nil {
		return errNilListenAndServeParameters
	}

	u.reporter = reporter

	buf := make([]byte, 65527) // max size for udp packet body (assuming ipv6)
	for {
		n, _, err := u.packetConn.ReadFrom(buf)
		if n > 0 {
			bufCopy := make([]byte, n)
			copy(bufCopy, buf)
			u.handlePacket(bufCopy, transferChan)
		}
		if err != nil {
			u.reporter.OnDebugf("UDP Transport (%s) - ReadFrom error: %v",
				u.packetConn.LocalAddr(),
				err)
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
	return u.packetConn.Close()
}

func (u *udpServer) handlePacket(
	data []byte,
	transferChan chan<- string,
) {
	buf := bytes.NewBuffer(data)
	for {
		bytes, err := buf.ReadBytes((byte)('\n'))
		if err == io.EOF {
			if len(bytes) == 0 {
				// Completed without errors.
				break
			}
		}
		line := strings.TrimSpace(string(bytes))
		if line != "" {
			transferChan <- line
		}
	}
}
