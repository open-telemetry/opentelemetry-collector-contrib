// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestInput(t *testing.T) {
	basicConfig := func() *syslog.Config {
		cfg := syslog.NewConfigWithID("test_syslog_parser")
		return cfg
	}

	cases, err := syslog.CreateCases(basicConfig)
	require.NoError(t, err)

	for _, tc := range cases {
		if tc.ValidForTCP {
			t.Run(fmt.Sprintf("TCP-%s", tc.Name), func(t *testing.T) {
				InputTest(t, NewConfigWithTCP(&tc.Config.BaseConfig), tc)
			})
		}

		if tc.ValidForUDP {
			t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
				InputTest(t, NewConfigWithUDP(&tc.Config.BaseConfig), tc)
			})
		}
	}
}

func InputTest(t *testing.T, cfg *Config, tc syslog.Case) {
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	ops := []operator.Operator{op, fake}
	p, err := pipeline.NewDirectedPipeline(ops)
	require.NoError(t, err)

	err = p.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)

	var conn net.Conn
	if cfg.TCP != nil {
		conn, err = net.Dial("tcp", cfg.TCP.ListenAddress)
		require.NoError(t, err)
	}
	if cfg.UDP != nil {
		conn, err = net.Dial("udp", cfg.UDP.ListenAddress)
		require.NoError(t, err)
	}

	if v, ok := tc.Input.Body.(string); ok {
		_, err = conn.Write([]byte(v))
	} else {
		_, err = conn.Write(tc.Input.Body.([]byte))
	}

	conn.Close()
	require.NoError(t, err)

	defer func() {
		require.NoError(t, p.Stop())
	}()
	select {
	case e := <-fake.Received:
		// close pipeline to avoid data race
		ots := time.Now()
		e.ObservedTimestamp = ots
		tc.Expect.ObservedTimestamp = ots
		require.Equal(t, tc.Expect, e)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be processed")
	}
}

func TestSyslogIDs(t *testing.T) {
	basicConfig := func() *syslog.BaseConfig {
		cfg := syslog.NewConfigWithID("test_syslog_parser")
		cfg.Protocol = "RFC3164"
		return &cfg.BaseConfig
	}

	t.Run("TCP", func(t *testing.T) {
		cfg := NewConfigWithTCP(basicConfig())
		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)
		syslogInputOp := op.(*Input)
		require.Equal(t, "test_syslog_internal_tcp", syslogInputOp.tcp.ID())
		require.Equal(t, "test_syslog_internal_parser", syslogInputOp.parser.ID())
		require.Equal(t, []string{syslogInputOp.parser.ID()}, syslogInputOp.tcp.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.parser.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.GetOutputIDs())
	})
	t.Run("UDP", func(t *testing.T) {
		cfg := NewConfigWithUDP(basicConfig())
		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)
		syslogInputOp := op.(*Input)
		require.Equal(t, "test_syslog_internal_udp", syslogInputOp.udp.ID())
		require.Equal(t, "test_syslog_internal_parser", syslogInputOp.parser.ID())
		require.Equal(t, []string{syslogInputOp.parser.ID()}, syslogInputOp.udp.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.parser.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.GetOutputIDs())
	})
}

func NewConfigWithTCP(syslogCfg *syslog.BaseConfig) *Config {
	cfg := NewConfigWithID("test_syslog")
	cfg.BaseConfig = *syslogCfg
	cfg.TCP = &tcp.NewConfigWithID("test_syslog_tcp").BaseConfig
	cfg.TCP.ListenAddress = ":14201"
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func NewConfigWithUDP(syslogCfg *syslog.BaseConfig) *Config {
	cfg := NewConfigWithID("test_syslog")
	cfg.BaseConfig = *syslogCfg
	cfg.UDP = &udp.NewConfigWithID("test_syslog_udp").BaseConfig
	cfg.UDP.ListenAddress = ":12032"
	cfg.OutputIDs = []string{"fake"}
	return cfg
}
