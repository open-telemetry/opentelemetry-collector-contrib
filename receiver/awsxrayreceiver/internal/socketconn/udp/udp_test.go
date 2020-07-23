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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUDPPortUnavailable(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sock, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	defer sock.Close()
	address := sock.LocalAddr().String()

	_, err = New(address)
	assert.Error(t, err, "should have failed to create a UDP listener")
	assert.Contains(t, err.Error(), "address already in use", "error message should complain about address in-use")
}

func TestSuccessfullyListen(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sock, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	address := sock.LocalAddr().String()
	sock.Close()

	_, err = New(address)
	assert.NoError(t, err, "should create a UDP listener")
}

func TestSuccessfullyRead(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sck, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	address := sck.LocalAddr().String()
	sck.Close()

	sock, err := New(address)
	assert.NoError(t, err, "should create a UDP listener")

	err = writePacket(t, address, "123")
	assert.NoError(t, err, "write should not return error")
	assert.Eventuallyf(t, func() bool {
		buf := make([]byte, 100)
		rlen, err := sock.Read(buf)
		assert.NoError(t, err, "Read should not return error")
		return rlen == 3
	}, time.Second, 10*time.Millisecond, "should read 3 bytes")
}

func TestSuccessfullyClose(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	assert.NoError(t, err, "should resolve UDP address")

	sck, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err, "should be able to listen")
	address := sck.LocalAddr().String()
	sck.Close()

	sock, err := New(address)
	assert.NoError(t, err, "should create a UDP listener")
	sock.Close()

	buf := make([]byte, 100)
	_, err = sock.Read(buf)
	assert.Error(t, err, "Read should fail after the listener is closed")
}

func writePacket(t *testing.T, addr, toWrite string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	n, err := fmt.Fprint(conn, toWrite)
	if err != nil {
		return err
	}
	assert.Equal(t, len(toWrite), n, "exunpected number of bytes written")
	return nil
}
