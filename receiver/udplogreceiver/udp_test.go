// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udplogreceiver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver/internal/metadata"
)

func TestUdp(t *testing.T) {
	listenAddress := "127.0.0.1:29018"
	testUDP(t, testdataConfigYaml(listenAddress), listenAddress)
}

func TestUdpAsync(t *testing.T) {
	listenAddress := "127.0.0.1:29019"
	cfg := testdataConfigYaml(listenAddress)
	cfg.InputConfig.AsyncConfig = &udp.AsyncConfig{
		Readers:        2,
		Processors:     2,
		MaxQueueLength: 100,
	}

	cfg.InputConfig.AsyncConfig.Readers = 2
	testUDP(t, testdataConfigYaml(listenAddress), listenAddress)
}

func testUDP(t *testing.T, cfg *UDPLogConfig, listenAddress string) {
	numLogs := 5

	f := NewFactory()
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	var conn net.Conn
	conn, err = net.Dial("udp", listenAddress)
	require.NoError(t, err)

	for i := range numLogs {
		msg := fmt.Sprintf("<86>1 2021-02-28T00:0%d:02.003Z test msg %d\n", i, i)
		_, err = conn.Write([]byte(msg))
		require.NoError(t, err)
	}
	require.NoError(t, conn.Close())

	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, time.Millisecond)
	require.NoError(t, rcvr.Shutdown(t.Context()))
	require.Len(t, sink.AllLogs(), 1)

	resourceLogs := sink.AllLogs()[0].ResourceLogs().At(0)
	logs := resourceLogs.ScopeLogs().At(0).LogRecords()
	require.Equal(t, logs.Len(), numLogs)

	expectedLogs := make([]string, numLogs)

	for i := range numLogs {
		expectedLogs[i] = fmt.Sprintf("<86>1 2021-02-28T00:0%d:02.003Z test msg %d", i, i)
	}

	for i := range numLogs {
		assert.Contains(t, expectedLogs, logs.At(i).Body().Str())
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub("udp_log")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, testdataConfigYaml("127.0.0.1:29018"), cfg)
}

func testdataConfigYaml(listenAddress string) *UDPLogConfig {
	return &UDPLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: func() udp.Config {
			c := udp.NewConfig()
			c.ListenAddress = listenAddress
			return *c
		}(),
	}
}

func TestDecodeInputConfigFailure(t *testing.T) {
	sink := new(consumertest.LogsSink)
	factory := NewFactory()
	badCfg := &UDPLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: func() udp.Config {
			c := udp.NewConfig()
			c.Encoding = "fake"
			return *c
		}(),
	}
	receiver, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), badCfg, sink)
	require.Error(t, err, "receiver creation should fail if input config isn't valid")
	require.Nil(t, receiver, "receiver creation should fail if input config isn't valid")
}

func expectNLogs(sink *consumertest.LogsSink, expected int) func() bool {
	return func() bool {
		return sink.LogRecordCount() == expected
	}
}

// ppv2Signature is the 12-byte fixed signature of a Proxy Protocol v2 header.
var ppv2Signature = []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}

// buildPPv2Datagram prepends a PPv2 header (IPv4 PROXY DGRAM) to payload.
func buildPPv2Datagram(srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16, payload []byte) []byte {
	var buf bytes.Buffer
	buf.Write(ppv2Signature)
	buf.WriteByte(0x21)                              // version 2 | PROXY command
	buf.WriteByte(0x12)                              // IPv4 family | DGRAM transport
	binary.Write(&buf, binary.BigEndian, uint16(12)) //nolint:errcheck // writing to a bytes.Buffer never fails
	buf.Write(srcIP.To4())
	buf.Write(dstIP.To4())
	binary.Write(&buf, binary.BigEndian, srcPort) //nolint:errcheck
	binary.Write(&buf, binary.BigEndian, dstPort) //nolint:errcheck
	buf.Write(payload)
	return buf.Bytes()
}

func TestProxyProtocolV2(t *testing.T) {
	listenAddress := "127.0.0.1:29020"

	cfg := testdataConfigYaml(listenAddress)
	cfg.InputConfig.ProxyProtocol = true
	cfg.InputConfig.AddAttributes = true

	f := NewFactory()
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, rcvr.Shutdown(t.Context()))
	}()

	conn, err := net.Dial("udp", listenAddress)
	require.NoError(t, err)
	defer conn.Close()

	proxiedSrcIP := net.ParseIP("1.2.3.4").To4()
	dstIP := net.ParseIP("127.0.0.1").To4()
	msg := []byte("<86>1 2021-02-28T00:00:02.003Z test proxied msg\n")
	datagram := buildPPv2Datagram(proxiedSrcIP, 5678, dstIP, 29020, msg)

	_, err = conn.Write(datagram)
	require.NoError(t, err)

	require.Eventually(t, expectNLogs(sink, 1), 2*time.Second, time.Millisecond)

	logRecord := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "<86>1 2021-02-28T00:00:02.003Z test proxied msg", logRecord.Body().Str())
	// The peer address should reflect the proxied source, not the raw UDP sender.
	assert.Equal(t, "1.2.3.4", logRecord.Attributes().AsRaw()["net.peer.ip"])
	assert.Equal(t, "5678", logRecord.Attributes().AsRaw()["net.peer.port"])
}
