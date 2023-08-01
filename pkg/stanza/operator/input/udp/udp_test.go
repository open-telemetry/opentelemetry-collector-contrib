// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func udpInputTest(input []byte, expected []string) func(t *testing.T) {
	return func(t *testing.T) {
		cfg := NewConfigWithID("test_input")
		cfg.ListenAddress = ":0"

		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*Input)
		require.True(t, ok)

		udpInput.InputOperator.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start(testutil.NewMockPersister("test"))
		require.NoError(t, err)
		defer func() {
			require.NoError(t, udpInput.Stop(), "expected to stop udp input operator without error")
		}()

		conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
		require.NoError(t, err)
		defer conn.Close()

		_, err = conn.Write(input)
		require.NoError(t, err)

		for _, expectedBody := range expected {
			select {
			case entry := <-entryChan:
				require.Equal(t, expectedBody, entry.Body)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for message to be written")
			}
		}

		select {
		case entry := <-entryChan:
			require.FailNow(t, "Unexpected entry: %s", entry)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func udpInputAttributesTest(input []byte, expected []string) func(t *testing.T) {
	return func(t *testing.T) {
		cfg := NewConfigWithID("test_input")
		cfg.ListenAddress = ":0"
		cfg.AddAttributes = true

		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*Input)
		require.True(t, ok)

		udpInput.InputOperator.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start(testutil.NewMockPersister("test"))
		require.NoError(t, err)
		defer func() {
			require.NoError(t, udpInput.Stop(), "expected to stop udp input operator without error")
		}()

		conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
		require.NoError(t, err)
		defer conn.Close()

		_, err = conn.Write(input)
		require.NoError(t, err)

		for _, expectedBody := range expected {
			select {
			case entry := <-entryChan:
				expectedAttributes := map[string]interface{}{
					"net.transport": "IP.UDP",
				}
				// LocalAddr for udpInput.connection is a server address
				if addr, ok := udpInput.connection.LocalAddr().(*net.UDPAddr); ok {
					ip := addr.IP.String()
					expectedAttributes["net.host.ip"] = addr.IP.String()
					expectedAttributes["net.host.port"] = strconv.FormatInt(int64(addr.Port), 10)
					expectedAttributes["net.host.name"] = udpInput.resolver.GetHostFromIP(ip)
				}
				// LocalAddr for conn is a client (peer) address
				if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
					ip := addr.IP.String()
					expectedAttributes["net.peer.ip"] = ip
					expectedAttributes["net.peer.port"] = strconv.FormatInt(int64(addr.Port), 10)
					expectedAttributes["net.peer.name"] = udpInput.resolver.GetHostFromIP(ip)
				}
				require.Equal(t, expectedBody, entry.Body)
				require.Equal(t, expectedAttributes, entry.Attributes)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for message to be written")
			}
		}

		select {
		case entry := <-entryChan:
			require.FailNow(t, "Unexpected entry: %s", entry)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func TestInput(t *testing.T) {
	t.Run("Simple", udpInputTest([]byte("message1"), []string{"message1"}))
	t.Run("TrailingNewlines", udpInputTest([]byte("message1\n"), []string{"message1"}))
	t.Run("TrailingCRNewlines", udpInputTest([]byte("message1\r\n"), []string{"message1"}))
	t.Run("NewlineInMessage", udpInputTest([]byte("message1\nmessage2\n"), []string{"message1\nmessage2"}))
}

func TestInputAttributes(t *testing.T) {
	t.Run("Simple", udpInputAttributesTest([]byte("message1"), []string{"message1"}))
	t.Run("TrailingNewlines", udpInputAttributesTest([]byte("message1\n"), []string{"message1"}))
	t.Run("TrailingCRNewlines", udpInputAttributesTest([]byte("message1\r\n"), []string{"message1"}))
	t.Run("NewlineInMessage", udpInputAttributesTest([]byte("message1\nmessage2\n"), []string{"message1\nmessage2"}))
}

func TestFailToBind(t *testing.T) {
	ip := "localhost"
	port := 0
	minPort := 30000
	maxPort := 40000
	for i := 1; 1 < 10; i++ {
		port = minPort + rand.Intn(maxPort-minPort+1)
		_, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), time.Second*2)
		if err != nil {
			// a failed connection indicates that the port is available for use
			break
		}
	}
	if port == 0 {
		t.Errorf("failed to find a free port between %d and %d", minPort, maxPort)
	}

	var startUDP = func(int) (*Input, error) {
		cfg := NewConfigWithID("test_input")
		cfg.ListenAddress = net.JoinHostPort(ip, strconv.Itoa(port))

		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*Input)
		require.True(t, ok)

		udpInput.InputOperator.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start(testutil.NewMockPersister("test"))
		return udpInput, err
	}

	first, err := startUDP(port)
	require.NoError(t, err, "expected first udp operator to start")
	defer func() {
		require.NoError(t, first.Stop(), "expected to stop udp input operator without error")
		require.NoError(t, first.Stop(), "expected stopping an already stopped operator to not return an error")
	}()
	_, err = startUDP(port)
	require.Error(t, err, "expected second udp operator to fail to start")
}

func BenchmarkUDPInput(b *testing.B) {
	cfg := NewConfigWithID("test_id")
	cfg.ListenAddress = ":0"

	op, err := cfg.Build(testutil.Logger(b))
	require.NoError(b, err)

	fakeOutput := testutil.NewFakeOutput(b)
	udpInput := op.(*Input)
	udpInput.InputOperator.OutputOperators = []operator.Operator{fakeOutput}

	err = udpInput.Start(testutil.NewMockPersister("test"))
	require.NoError(b, err)

	done := make(chan struct{})
	go func() {
		conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
		require.NoError(b, err)
		defer func() {
			require.NoError(b, udpInput.Stop())
		}()
		defer conn.Close()
		message := []byte("message\n")
		for {
			select {
			case <-done:
				return
			default:
				_, err := conn.Write(message)
				require.NoError(b, err)
			}
		}
	}()

	for i := 0; i < b.N; i++ {
		<-fakeOutput.Received
	}

	defer close(done)
}
