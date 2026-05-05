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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
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

func TestSyslogAutoWithTcp(t *testing.T) {
	testAutoSyslog(t, testdataAutoTCPConfig())
}

func TestSyslogAutoWithUdp(t *testing.T) {
	testAutoSyslog(t, testdataAutoUDPConfig())
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

func testAutoSyslog(t *testing.T, cfg *SysLogConfig) {
	f := NewFactory()
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	address := "127.0.0.1:29019"
	network := "tcp"
	if cfg.InputConfig.UDP != nil {
		address = "127.0.0.1:29020"
		network = "udp"
	}

	conn, err := net.Dial(network, address)
	require.NoError(t, err)

	rfc3164Message := fmt.Sprintf("<34>%s 1.2.3.4 apache_server: legacy msg", time.Now().Format("Jan _2 15:04:05"))
	messages := []string{
		"<86>1 2021-02-28T00:00:02.003Z 192.168.1.1 SecureAuth0 23108 ID52020 - modern msg",
		rfc3164Message,
	}

	for _, msg := range messages {
		if network == "tcp" {
			msg += "\n"
		}
		_, err = conn.Write([]byte(msg))
		require.NoError(t, err)
	}
	require.NoError(t, conn.Close())

	require.Eventually(t, expectNLogs(sink, len(messages)), 2*time.Second, time.Millisecond)
	require.NoError(t, rcvr.Shutdown(t.Context()))

	seen := map[string]bool{}
	for _, logs := range sink.AllLogs() {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			scopeLogs := logs.ResourceLogs().At(i).ScopeLogs()
			for j := 0; j < scopeLogs.Len(); j++ {
				records := scopeLogs.At(j).LogRecords()
				for k := 0; k < records.Len(); k++ {
					message, ok := records.At(k).Attributes().AsRaw()["message"].(string)
					require.True(t, ok)
					seen[message] = true
				}
			}
		}
	}

	require.Equal(t, map[string]bool{
		"modern msg": true,
		"legacy msg": true,
	}, seen)
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub("syslog")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, testdataConfigYaml(), cfg)
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

func testdataAutoTCPConfig() *SysLogConfig {
	return &SysLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: func() syslog.Config {
			c := syslog.NewConfig()
			c.TCP = &tcp.NewConfig().BaseConfig
			c.TCP.ListenAddress = "127.0.0.1:29019"
			c.Protocol = "auto"
			return *c
		}(),
	}
}

func testdataAutoUDPConfig() *SysLogConfig {
	return &SysLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{},
		},
		InputConfig: func() syslog.Config {
			c := syslog.NewConfig()
			c.UDP = &udp.NewConfig().BaseConfig
			c.UDP.ListenAddress = "127.0.0.1:29020"
			c.Protocol = "auto"
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
