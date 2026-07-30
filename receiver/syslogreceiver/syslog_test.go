// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogreceiver

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
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

func TestUseSynchronousLogEmitter(t *testing.T) {
	receiverType := ReceiverType{}
	assert.True(t, receiverType.UseSynchronousLogEmitter(testdataConfigYaml()))
	assert.False(t, receiverType.UseSynchronousLogEmitter(testdataUDPConfig()))
}

func TestSyslogWithTCPBackpressure(t *testing.T) {
	consumeStarted := make(chan int, 2)
	releaseConsumer := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseConsumer)
		})
	}
	nextConsumer, err := consumer.NewLogs(func(ctx context.Context, logs plog.Logs) error {
		consumeStarted <- logs.LogRecordCount()
		select {
		case <-releaseConsumer:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	require.NoError(t, err)

	cfg := testdataConfigYaml()
	cfg.InputConfig.TCP.ConnectionIdleTimeout = 50 * time.Millisecond
	factory := NewFactory()
	rcvr, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nextConsumer)
	require.NoError(t, err)
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		release()
		shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		assert.NoError(t, rcvr.Shutdown(shutdownCtx))
	})

	conn, err := net.Dial("tcp", "127.0.0.1:29018")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	const message = "<86>1 2021-02-28T00:00:02.003Z 192.168.1.1 SecureAuth0 23108 ID52020 [SecureAuth@27389] test msg\n"
	_, err = fmt.Fprint(conn, message)
	require.NoError(t, err)

	select {
	case count := <-consumeStarted:
		require.Equal(t, 1, count)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for the first log")
	}

	_, err = fmt.Fprint(conn, message)
	require.NoError(t, err)
	require.Never(t, func() bool {
		return len(consumeStarted) > 0
	}, 200*time.Millisecond, 10*time.Millisecond)

	release()
	select {
	case count := <-consumeStarted:
		require.Equal(t, 1, count)
	case <-time.After(time.Second):
		require.FailNow(t, "timed out waiting for the second log")
	}
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

	logs := make([]plog.LogRecord, 0, numLogs)
	for _, receivedLogs := range sink.AllLogs() {
		resourceLogs := receivedLogs.ResourceLogs().At(0)
		logRecords := resourceLogs.ScopeLogs().At(0).LogRecords()
		for i := range logRecords.Len() {
			logs = append(logs, logRecords.At(i))
		}
	}
	require.Len(t, logs, numLogs)

	for i := range numLogs {
		log := logs[i]

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
