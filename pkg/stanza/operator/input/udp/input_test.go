// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"fmt"
	"math/rand/v2"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func udpInputTest(input []byte, expected []string, cfg *Config) func(t *testing.T) {
	return func(t *testing.T) {
		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*Input)
		require.True(t, ok)

		udpInput.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start(testutil.NewUnscopedMockPersister())
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
			require.FailNow(t, fmt.Sprintf("Unexpected entry: %s", entry))
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func udpInputAttributesTest(input []byte, expected []string, useStableNetworkAttributes bool) func(t *testing.T) {
	return func(t *testing.T) {
		if useStableNetworkAttributes {
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.StanzaUDPUseStableNetworkAttributesFeatureGate.ID(), true))
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.StanzaUDPUseStableNetworkAttributesFeatureGate.ID(), false))
			})
		}

		cfg := NewConfigWithID("test_input")
		cfg.ListenAddress = ":0"
		cfg.AddAttributes = true

		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*Input)
		require.True(t, ok)

		udpInput.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start(testutil.NewUnscopedMockPersister())
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
				expectedAttributes := map[string]any{}
				if useStableNetworkAttributes {
					expectedAttributes["network.transport"] = "udp"
					// LocalAddr for udpInput.connection is a server address
					if addr, ok := udpInput.connection.LocalAddr().(*net.UDPAddr); ok {
						ip := addr.IP.String()
						expectedAttributes["network.local.address"] = ip
						expectedAttributes["server.port"] = strconv.FormatInt(int64(addr.Port), 10)
						expectedAttributes["server.address"] = udpInput.resolver.GetHostFromIP(ip)
					}
					// LocalAddr for conn is a client (peer) address
					if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
						ip := addr.IP.String()
						expectedAttributes["network.peer.address"] = ip
						expectedAttributes["client.port"] = strconv.FormatInt(int64(addr.Port), 10)
						expectedAttributes["client.address"] = udpInput.resolver.GetHostFromIP(ip)
					}
				} else {
					expectedAttributes["net.transport"] = "IP.UDP"
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
				}
				require.Equal(t, expectedBody, entry.Body)
				require.Equal(t, expectedAttributes, entry.Attributes)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for message to be written")
			}
		}

		select {
		case entry := <-entryChan:
			require.FailNow(t, fmt.Sprintf("Unexpected entry: %s", entry))
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

func TestInput(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = ":0"

	t.Run("Simple", udpInputTest([]byte("message1"), []string{"message1"}, cfg))
	t.Run("TrailingNewlines", udpInputTest([]byte("message1\n"), []string{"message1"}, cfg))
	t.Run("TrailingCRNewlines", udpInputTest([]byte("message1\r\n"), []string{"message1"}, cfg))
	t.Run("NewlineInMessage", udpInputTest([]byte("message1\nmessage2\n"), []string{"message1\nmessage2"}, cfg))

	cfg.AsyncConfig = &AsyncConfig{
		Readers:        2,
		Processors:     2,
		MaxQueueLength: 100,
	}
	t.Run("SimpleAsync", udpInputTest([]byte("message1"), []string{"message1"}, cfg))
}

func TestInputAttributes(t *testing.T) {
	t.Run("Simple", udpInputAttributesTest([]byte("message1"), []string{"message1"}, false))
	t.Run("TrailingNewlines", udpInputAttributesTest([]byte("message1\n"), []string{"message1"}, false))
	t.Run("TrailingCRNewlines", udpInputAttributesTest([]byte("message1\r\n"), []string{"message1"}, false))
	t.Run("NewlineInMessage", udpInputAttributesTest([]byte("message1\nmessage2\n"), []string{"message1\nmessage2"}, false))
}

func TestInputStableNetworkAttributes(t *testing.T) {
	t.Run("Simple", udpInputAttributesTest([]byte("message1"), []string{"message1"}, true))
	t.Run("TrailingNewlines", udpInputAttributesTest([]byte("message1\n"), []string{"message1"}, true))
	t.Run("TrailingCRNewlines", udpInputAttributesTest([]byte("message1\r\n"), []string{"message1"}, true))
	t.Run("NewlineInMessage", udpInputAttributesTest([]byte("message1\nmessage2\n"), []string{"message1\nmessage2"}, true))
}

func TestFailToBind(t *testing.T) {
	ip := "localhost"
	port := 0
	minPort := 30000
	maxPort := 40000
	for i := 1; 1 < 10; i++ {
		port = minPort + rand.IntN(maxPort-minPort+1)
		_, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), time.Second*2)
		if err != nil {
			// a failed connection indicates that the port is available for use
			break
		}
	}
	if port == 0 {
		t.Errorf("failed to find a free port between %d and %d", minPort, maxPort)
	}

	startUDP := func(int) (*Input, error) {
		cfg := NewConfigWithID("test_input")
		cfg.ListenAddress = net.JoinHostPort(ip, strconv.Itoa(port))

		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		udpInput, ok := op.(*Input)
		require.True(t, ok)

		udpInput.OutputOperators = []operator.Operator{&mockOutput}

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = udpInput.Start(testutil.NewUnscopedMockPersister())
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

type mockResolver struct {
	calls     atomic.Int64
	blockChan chan struct{}
	hostMap   map[string]string
}

func (m *mockResolver) GetHostFromIP(ip string) string {
	m.calls.Add(1)
	if m.blockChan != nil {
		<-m.blockChan
	}
	if m.hostMap != nil {
		if host, ok := m.hostMap[ip]; ok {
			return host
		}
	}
	return "mocked-host.example.com"
}

func (*mockResolver) Stop() {}

func TestInputResolveNamesFalse(t *testing.T) {
	for _, useStable := range []bool{false, true} {
		testName := "DeprecatedAttributes"
		if useStable {
			testName = "StableAttributes"
		}
		t.Run(testName, func(t *testing.T) {
			if useStable {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.StanzaUDPUseStableNetworkAttributesFeatureGate.ID(), true))
				t.Cleanup(func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(metadata.StanzaUDPUseStableNetworkAttributesFeatureGate.ID(), false))
				})
			}

			cfg := NewConfigWithID("test_input")
			cfg.ListenAddress = "127.0.0.1:0"
			cfg.AddAttributes = true
			cfg.ResolveNames = configoptional.Some(false)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			mockOutput := testutil.Operator{}
			udpInput, ok := op.(*Input)
			require.True(t, ok)
			require.Nil(t, udpInput.resolver, "resolver should not be initialized when resolve_names is false")

			udpInput.OutputOperators = []operator.Operator{&mockOutput}

			entryChan := make(chan *entry.Entry, 1)
			mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				entryChan <- args.Get(1).(*entry.Entry)
			}).Return(nil)

			err = udpInput.Start(testutil.NewUnscopedMockPersister())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, udpInput.Stop())
			}()

			conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
			require.NoError(t, err)
			defer conn.Close()

			_, err = conn.Write([]byte("test log\n"))
			require.NoError(t, err)

			select {
			case entry := <-entryChan:
				require.Equal(t, "test log", entry.Body)
				if useStable {
					assert.Equal(t, "udp", entry.Attributes["network.transport"])
					assert.Equal(t, "127.0.0.1", entry.Attributes["network.local.address"])
					assert.NotEmpty(t, entry.Attributes["server.port"])
					assert.Equal(t, "127.0.0.1", entry.Attributes["network.peer.address"])
					assert.NotEmpty(t, entry.Attributes["client.port"])
					assert.NotContains(t, entry.Attributes, "server.address", "server.address should be omitted when resolve_names is false")
					assert.NotContains(t, entry.Attributes, "client.address", "client.address should be omitted when resolve_names is false")
				} else {
					assert.Equal(t, "IP.UDP", entry.Attributes["net.transport"])
					assert.Equal(t, "127.0.0.1", entry.Attributes["net.host.ip"])
					assert.NotEmpty(t, entry.Attributes["net.host.port"])
					assert.Equal(t, "127.0.0.1", entry.Attributes["net.peer.ip"])
					assert.NotEmpty(t, entry.Attributes["net.peer.port"])
					assert.NotContains(t, entry.Attributes, "net.host.name", "net.host.name should be omitted when resolve_names is false")
					assert.NotContains(t, entry.Attributes, "net.peer.name", "net.peer.name should be omitted when resolve_names is false")
				}
			case <-time.After(2 * time.Second):
				require.FailNow(t, "Timed out waiting for entry")
			}
		})
	}
}

func TestInputZeroDNSCallsWhenResolveNamesFalse(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AddAttributes = true
	cfg.ResolveNames = configoptional.Some(false)

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.Operator{}
	udpInput, ok := op.(*Input)
	require.True(t, ok)

	if udpInput.resolver != nil {
		udpInput.resolver.Stop()
	}
	mockRes := &mockResolver{}
	udpInput.resolver = mockRes
	udpInput.OutputOperators = []operator.Operator{&mockOutput}

	entryChan := make(chan *entry.Entry, 10)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entryChan <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, udpInput.Stop())
	}()

	conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()

	for i := range 5 {
		_, err = fmt.Fprintf(conn, "msg %d\n", i)
		require.NoError(t, err)
	}

	for i := range 5 {
		select {
		case entry := <-entryChan:
			require.Equal(t, fmt.Sprintf("msg %d", i), entry.Body)
		case <-time.After(2 * time.Second):
			require.FailNow(t, "Timed out waiting for entry")
		}
	}

	assert.Equal(t, int64(0), mockRes.calls.Load(), "Expected zero reverse DNS calls when resolve_names is false")
}

func TestInputSlowResolverDoesNotBlockWhenResolveNamesFalse(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AddAttributes = true
	cfg.ResolveNames = configoptional.Some(false)

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.Operator{}
	udpInput, ok := op.(*Input)
	require.True(t, ok)

	// mock resolver with a blockChan to simulate an unresponsive resolver
	blockChan := make(chan struct{})
	mockRes := &mockResolver{blockChan: blockChan}
	udpInput.resolver = mockRes
	udpInput.OutputOperators = []operator.Operator{&mockOutput}

	entryChan := make(chan *entry.Entry, 1)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entryChan <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	defer func() {
		close(blockChan)
		require.NoError(t, udpInput.Stop())
	}()

	conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("fast log\n"))
	require.NoError(t, err)

	select {
	case entry := <-entryChan:
		require.Equal(t, "fast log", entry.Body)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Timed out waiting for fast log; resolver blocked when it should not have")
	}

	assert.Equal(t, int64(0), mockRes.calls.Load(), "Should not invoke slow resolver when resolve_names is false")
}

func TestInputSlowResolverBlocksWhenResolveNamesTrue(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AddAttributes = true
	cfg.ResolveNames = configoptional.Some(true)

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.Operator{}
	udpInput, ok := op.(*Input)
	require.True(t, ok)

	if udpInput.resolver != nil {
		udpInput.resolver.Stop()
	}
	blockChan := make(chan struct{})
	mockRes := &mockResolver{blockChan: blockChan}
	udpInput.resolver = mockRes
	udpInput.OutputOperators = []operator.Operator{&mockOutput}

	entryChan := make(chan *entry.Entry, 1)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entryChan <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, udpInput.Stop())
	}()

	conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("blocked log\n"))
	require.NoError(t, err)

	// Ensure no message arrives while blocked
	select {
	case <-entryChan:
		require.FailNow(t, "Message should not arrive while resolver is blocked")
	case <-time.After(100 * time.Millisecond):
		// Expected: still waiting on resolver
	}

	assert.GreaterOrEqual(t, mockRes.calls.Load(), int64(1), "Resolver should have been called")

	// Now unblock resolver
	close(blockChan)

	select {
	case entry := <-entryChan:
		require.Equal(t, "blocked log", entry.Body)
		assert.Equal(t, "mocked-host.example.com", entry.Attributes["net.peer.name"])
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Timed out waiting for message after resolver unblocked")
	}
}

func TestInputResolveNamesTrue(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AddAttributes = true
	cfg.ResolveNames = configoptional.Some(true)

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.Operator{}
	udpInput, ok := op.(*Input)
	require.True(t, ok)

	if udpInput.resolver != nil {
		udpInput.resolver.Stop()
	}
	mockRes := &mockResolver{
		hostMap: map[string]string{
			"127.0.0.1": "custom.resolved.host",
		},
	}
	udpInput.resolver = mockRes
	udpInput.OutputOperators = []operator.Operator{&mockOutput}

	entryChan := make(chan *entry.Entry, 1)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entryChan <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, udpInput.Stop())
	}()

	conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("resolvable log\n"))
	require.NoError(t, err)

	select {
	case entry := <-entryChan:
		require.Equal(t, "resolvable log", entry.Body)
		assert.Equal(t, "custom.resolved.host", entry.Attributes["net.peer.name"])
		assert.Equal(t, "127.0.0.1", entry.Attributes["net.peer.ip"])
		assert.NotEmpty(t, entry.Attributes["net.peer.port"])
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Timed out waiting for message")
	}
}

func TestInputDNSUnresolvable(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AddAttributes = true
	cfg.ResolveNames = configoptional.Some(true)

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.Operator{}
	udpInput, ok := op.(*Input)
	require.True(t, ok)

	if udpInput.resolver != nil {
		udpInput.resolver.Stop()
	}
	// Simulating unresolvable IP where resolver returns IP as fallback
	mockRes := &mockResolver{
		hostMap: map[string]string{
			"127.0.0.1": "127.0.0.1",
		},
	}
	udpInput.resolver = mockRes
	udpInput.OutputOperators = []operator.Operator{&mockOutput}

	entryChan := make(chan *entry.Entry, 1)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entryChan <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, udpInput.Stop())
	}()

	conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("unresolvable log\n"))
	require.NoError(t, err)

	select {
	case entry := <-entryChan:
		require.Equal(t, "unresolvable log", entry.Body)
		assert.Equal(t, "127.0.0.1", entry.Attributes["net.peer.name"])
		assert.Equal(t, "127.0.0.1", entry.Attributes["net.peer.ip"])
	case <-time.After(2 * time.Second):
		require.FailNow(t, "Timed out waiting for message")
	}
}

func TestInputDNSCaching(t *testing.T) {
	cfg := NewConfigWithID("test_input")
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.AddAttributes = true

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	mockOutput := testutil.Operator{}
	udpInput, ok := op.(*Input)
	require.True(t, ok)
	require.NotNil(t, udpInput.resolver, "standard resolver should be initialized")

	udpInput.OutputOperators = []operator.Operator{&mockOutput}

	entryChan := make(chan *entry.Entry, 10)
	mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		entryChan <- args.Get(1).(*entry.Entry)
	}).Return(nil)

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, udpInput.Stop())
	}()

	conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
	require.NoError(t, err)
	defer conn.Close()

	for i := range 5 {
		_, err = fmt.Fprintf(conn, "repeated msg %d\n", i)
		require.NoError(t, err)
	}

	for i := range 5 {
		select {
		case entry := <-entryChan:
			require.Equal(t, fmt.Sprintf("repeated msg %d", i), entry.Body)
			assert.NotEmpty(t, entry.Attributes["net.peer.name"])
			assert.Equal(t, "127.0.0.1", entry.Attributes["net.peer.ip"])
		case <-time.After(2 * time.Second):
			require.FailNow(t, "Timed out waiting for entry")
		}
	}
}

func TestInputIPv6(t *testing.T) {
	testConn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback, Port: 0})
	if err != nil {
		t.Skip("IPv6 not supported on this host")
	}
	_ = testConn.Close()

	for _, resolve := range []bool{false, true} {
		testName := "ResolveNamesFalse"
		if resolve {
			testName = "ResolveNamesTrue"
		}
		t.Run(testName, func(t *testing.T) {
			cfg := NewConfigWithID("test_input")
			cfg.ListenAddress = "[::1]:0"
			cfg.AddAttributes = true
			cfg.ResolveNames = configoptional.Some(resolve)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			mockOutput := testutil.Operator{}
			udpInput, ok := op.(*Input)
			require.True(t, ok)

			if resolve {
				if udpInput.resolver != nil {
					udpInput.resolver.Stop()
				}
				mockRes := &mockResolver{
					hostMap: map[string]string{
						"::1": "ipv6.example.com",
					},
				}
				udpInput.resolver = mockRes
			}

			udpInput.OutputOperators = []operator.Operator{&mockOutput}

			entryChan := make(chan *entry.Entry, 1)
			mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				entryChan <- args.Get(1).(*entry.Entry)
			}).Return(nil)

			err = udpInput.Start(testutil.NewUnscopedMockPersister())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, udpInput.Stop())
			}()

			conn, err := net.Dial("udp6", udpInput.connection.LocalAddr().String())
			require.NoError(t, err)
			defer conn.Close()

			_, err = conn.Write([]byte("ipv6 message\n"))
			require.NoError(t, err)

			select {
			case entry := <-entryChan:
				require.Equal(t, "ipv6 message", entry.Body)
				assert.Equal(t, "::1", entry.Attributes["net.peer.ip"])
				assert.NotEmpty(t, entry.Attributes["net.peer.port"])
				if resolve {
					assert.Equal(t, "ipv6.example.com", entry.Attributes["net.peer.name"])
				} else {
					assert.NotContains(t, entry.Attributes, "net.peer.name")
				}
			case <-time.After(2 * time.Second):
				require.FailNow(t, "Timed out waiting for IPv6 message")
			}
		})
	}
}

func BenchmarkUDPInput(b *testing.B) {
	cfg := NewConfigWithID("test_id")
	cfg.ListenAddress = ":0"

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(b, err)

	fakeOutput := testutil.NewFakeOutput(b)
	udpInput := op.(*Input)
	udpInput.OutputOperators = []operator.Operator{fakeOutput}

	err = udpInput.Start(testutil.NewUnscopedMockPersister())
	require.NoError(b, err)

	done := make(chan struct{})
	go func() {
		conn, err := net.Dial("udp", udpInput.connection.LocalAddr().String())
		assert.NoError(b, err)
		defer func() {
			assert.NoError(b, udpInput.Stop())
		}()
		defer conn.Close()
		message := []byte("message\n")
		for {
			select {
			case <-done:
				return
			default:
				_, err := conn.Write(message)
				assert.NoError(b, err)
			}
		}
	}()

	for b.Loop() {
		<-fakeOutput.Received
	}

	close(done)
}
