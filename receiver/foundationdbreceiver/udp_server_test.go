// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
