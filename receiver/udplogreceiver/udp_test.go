// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udplogreceiver

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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

	assert.NoError(t, confmap.Validate(cfg))
	assert.Equal(t, testdataConfigYaml("127.0.0.1:29018"), cfg)

	// test resolve_names: false
	cfgFalse := factory.CreateDefaultConfig()
	subFalse, err := cm.Sub("udp_log/resolve_names_false")
	require.NoError(t, err)
	require.NoError(t, subFalse.Unmarshal(cfgFalse))
	assert.NoError(t, confmap.Validate(cfgFalse))
	expectedFalse := testdataConfigYaml("127.0.0.1:29018")
	expectedFalse.InputConfig.AddAttributes = true
	expectedFalse.InputConfig.ResolveNames = configoptional.Some(false)
	assert.Equal(t, expectedFalse, cfgFalse)

	// test resolve_names: true
	cfgTrue := factory.CreateDefaultConfig()
	subTrue, err := cm.Sub("udp_log/resolve_names_true")
	require.NoError(t, err)
	require.NoError(t, subTrue.Unmarshal(cfgTrue))
	assert.NoError(t, confmap.Validate(cfgTrue))
	expectedTrue := testdataConfigYaml("127.0.0.1:29018")
	expectedTrue.InputConfig.AddAttributes = true
	expectedTrue.InputConfig.ResolveNames = configoptional.Some(true)
	assert.Equal(t, expectedTrue, cfgTrue)
}

func TestUdpResolveNamesFalse(t *testing.T) {
	listenAddress := "127.0.0.1:29020"
	cfg := testdataConfigYaml(listenAddress)
	cfg.InputConfig.AddAttributes = true
	cfg.InputConfig.ResolveNames = configoptional.Some(false)

	numLogs := 3
	f := NewFactory()
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	conn, err := net.Dial("udp", listenAddress)
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

	for i := range numLogs {
		attrs := logs.At(i).Attributes().AsRaw()
		assert.Equal(t, "IP.UDP", attrs["net.transport"])
		assert.Equal(t, "127.0.0.1", attrs["net.host.ip"])
		assert.NotEmpty(t, attrs["net.host.port"])
		assert.Equal(t, "127.0.0.1", attrs["net.peer.ip"])
		assert.NotEmpty(t, attrs["net.peer.port"])
		assert.NotContains(t, attrs, "net.host.name", "net.host.name must not be present when resolve_names is false")
		assert.NotContains(t, attrs, "net.peer.name", "net.peer.name must not be present when resolve_names is false")
	}
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
