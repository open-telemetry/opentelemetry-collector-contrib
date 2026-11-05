// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogreceiver

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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver/internal/metadata"
)

func TestSyslogWithTcp(t *testing.T) {
	testSyslog(t, testdataConfigYaml())
}

func TestSyslogWithUdp(t *testing.T) {
	testSyslog(t, testdataUDPConfig())
}

func testSyslog(t *testing.T, cfg *SysLogConfig) {
	numLogs := 5

	f := NewFactory()
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	var conn net.Conn
	if cfg.InputConfig.TCP != nil {
		conn, err = net.Dial("tcp", "127.0.0.1:29018")
		require.NoError(t, err)
	} else {
		conn, err = net.Dial("udp", "127.0.0.1:29018")
		require.NoError(t, err)
	}

	for i := range numLogs {
		msg := fmt.Sprintf("<86>1 2021-02-28T00:0%d:02.003Z 192.168.1.1 SecureAuth0 23108 ID52020 [SecureAuth@27389] test msg %d\n", i, i)
		_, err = conn.Write([]byte(msg))
		require.NoError(t, err)
	}
	require.NoError(t, conn.Close())

	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, time.Millisecond)
	require.NoError(t, rcvr.Shutdown(t.Context()))
	require.Len(t, sink.AllLogs(), 1)

	resourceLogs := sink.AllLogs()[0].ResourceLogs().At(0)
	logs := resourceLogs.ScopeLogs().At(0).LogRecords()

	for i := range numLogs {
		log := logs.At(i)

		require.Equal(t, log.Timestamp(), pcommon.Timestamp(1614470402003000000+i*60*1000*1000*1000))
		msg, ok := log.Attributes().AsRaw()["message"]
		require.True(t, ok)
		require.Equal(t, msg, fmt.Sprintf("test msg %d", i))
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub("syslog")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, confmap.Validate(cfg))
	assert.Equal(t, testdataConfigYaml(), cfg)

	// test syslog/udp_resolve_names_false unmarshaling
	cfgUDPFalse := factory.CreateDefaultConfig()
	subUDPFalse, err := cm.Sub("syslog/udp_resolve_names_false")
	require.NoError(t, err)
	require.NoError(t, subUDPFalse.Unmarshal(cfgUDPFalse))
	assert.NoError(t, confmap.Validate(cfgUDPFalse))
	expectedUDPFalse := testdataUDPConfig()
	expectedUDPFalse.RetryOnFailure = consumerretry.NewDefaultConfig()
	expectedUDPFalse.InputConfig.UDP.ListenAddress = "127.0.0.1:29022"
	expectedUDPFalse.InputConfig.UDP.AddAttributes = true
	expectedUDPFalse.InputConfig.UDP.ResolveNames = configoptional.Some(false)
	assert.Equal(t, expectedUDPFalse, cfgUDPFalse)
}

func TestSyslogWithUdpResolveNamesFalse(t *testing.T) {
	listenAddress := "127.0.0.1:29022"
	cfg := testdataUDPConfig()
	cfg.InputConfig.UDP.ListenAddress = listenAddress
	cfg.InputConfig.UDP.AddAttributes = true
	cfg.InputConfig.UDP.ResolveNames = configoptional.Some(false)

	numLogs := 3
	f := NewFactory()
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	conn, err := net.Dial("udp", listenAddress)
	require.NoError(t, err)

	for i := range numLogs {
		msg := fmt.Sprintf("<86>1 2021-02-28T00:0%d:02.003Z 192.168.1.1 SecureAuth0 23108 ID52020 [SecureAuth@27389] test msg %d\n", i, i)
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
		log := logs.At(i)
		require.Equal(t, log.Timestamp(), pcommon.Timestamp(1614470402003000000+i*60*1000*1000*1000))
		attrs := log.Attributes().AsRaw()

		// Verify syslog parsing works
		assert.Equal(t, fmt.Sprintf("test msg %d", i), attrs["message"])
		assert.Equal(t, "SecureAuth0", attrs["appname"])
		assert.Equal(t, "192.168.1.1", attrs["hostname"])
		assert.Equal(t, int64(86), attrs["priority"])
		assert.Equal(t, int64(10), attrs["facility"])

		// Verify network attributes are present
		assert.Equal(t, "IP.UDP", attrs["net.transport"])
		assert.Equal(t, "127.0.0.1", attrs["net.peer.ip"])
		assert.NotEmpty(t, attrs["net.peer.port"])
		assert.Equal(t, "127.0.0.1", attrs["net.host.ip"])
		assert.NotEmpty(t, attrs["net.host.port"])

		// Verify reverse DNS is not invoked and hostname attributes are absent
		assert.NotContains(t, attrs, "net.peer.name")
		assert.NotContains(t, attrs, "net.host.name")
	}
}

func testdataConfigYaml() *SysLogConfig {
	return &SysLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() syslog.Config {
			c := syslog.NewConfig()
			c.TCP = &tcp.NewConfig().BaseConfig
			c.TCP.ListenAddress = "127.0.0.1:29018"
			c.Protocol = "rfc5424"
			return *c
		}(),
	}
}

func testdataUDPConfig() *SysLogConfig {
	return &SysLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: func() syslog.Config {
			c := syslog.NewConfig()
			c.UDP = &udp.NewConfig().BaseConfig
			c.UDP.ListenAddress = "127.0.0.1:29018"
			c.Protocol = "rfc5424"
			return *c
		}(),
	}
}

func TestDecodeInputConfigFailure(t *testing.T) {
	sink := new(consumertest.LogsSink)
	factory := NewFactory()
	badCfg := &SysLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: func() syslog.Config {
			c := syslog.NewConfig()
			c.TCP = &tcp.NewConfig().BaseConfig
			c.Protocol = "fake"
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
