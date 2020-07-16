// Copyright 2019, OpenTelemetry Authors
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

package udp

import (
	"net"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/socketconn"
)

// Reference for this port:
// https://github.com/aws/aws-xray-daemon/blob/master/pkg/socketconn/udp/udp.go

// UDP defines UDP socket connection.
type UDP struct {
	socket *net.UDPConn
}

// New returns new instance of UDP.
func New(udpAddress string) (socketconn.SocketConn, error) {
	addr, err := net.ResolveUDPAddr("udp", udpAddress)
	if err != nil {
		return nil, err
	}
	sock, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return UDP{
		socket: sock,
	}, nil
}

// Read returns number of bytes read from the UDP connection.
func (conn UDP) Read(b []byte) (int, error) {
	rlen, _, err := conn.socket.ReadFromUDP(b)
	return rlen, err
}

// Close closes current UDP connection.
func (conn UDP) Close() {
	conn.socket.Close()
}
