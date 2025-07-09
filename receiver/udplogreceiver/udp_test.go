// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udplogreceiver

import (
	"context"
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
	rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	var conn net.Conn
	conn, err = net.Dial("udp", listenAddress)
	require.NoError(t, err)

	for i := 0; i < numLogs; i++ {
		msg := fmt.Sprintf("<86>1 2021-02-28T00:0%d:02.003Z test msg %d\n", i, i)
		_, err = conn.Write([]byte(msg))
		require.NoError(t, err)
	}
	require.NoError(t, conn.Close())

	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, time.Millisecond)
	require.NoError(t, rcvr.Shutdown(context.Background()))
	require.Len(t, sink.AllLogs(), 1)

	resourceLogs := sink.AllLogs()[0].ResourceLogs().At(0)
	logs := resourceLogs.ScopeLogs().At(0).LogRecords()
	require.Equal(t, logs.Len(), numLogs)

	expectedLogs := make([]string, numLogs)

	for i := 0; i < numLogs; i++ {
		expectedLogs[i] = fmt.Sprintf("<86>1 2021-02-28T00:0%d:02.003Z test msg %d", i, i)
	}

	for i := 0; i < numLogs; i++ {
		assert.Contains(t, expectedLogs, logs.At(i).Body().Str())
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub("udplog")
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
	receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), badCfg, sink)
	require.Error(t, err, "receiver creation should fail if input config isn't valid")
	require.Nil(t, receiver, "receiver creation should fail if input config isn't valid")
}

func expectNLogs(sink *consumertest.LogsSink, expected int) func() bool {
	return func() bool {
		return sink.LogRecordCount() == expected
	}
}
