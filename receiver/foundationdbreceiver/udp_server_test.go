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

package foundationdbreceiver

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockFDBTraceHandler struct {
	err error
}

func (mh *MockFDBTraceHandler) Handle(data []byte) error {
	return mh.err
}

func TestUDP(t *testing.T) {
	udpServer, err := NewUDPServer(defaultAddress, defaultSocketBufferSize)
	require.NoError(t, err)
	require.NotNil(t, udpServer)
    udpServer.Close()
}

func TestUDPWithHandlerErr(t *testing.T) {
	udpServer, err := NewUDPServer(defaultAddress, defaultSocketBufferSize)
	require.NoError(t, err)
	require.NotNil(t, udpServer)

	handler := &MockFDBTraceHandler{err: fmt.Errorf("FAIL")}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err := udpServer.ListenAndServe(handler, defaultMaxPacketSize)
		require.Error(t, err, "expected error")
        assert.Equal(t, "FAIL", err.Error())
		wg.Done()
	}()

	var conn net.Conn
	conn, err = net.Dial("udp", defaultAddress)
	require.NoError(t, err)

    _, err = conn.Write([]byte("foo"))
    wg.Wait()
}
