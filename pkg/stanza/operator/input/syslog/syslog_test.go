// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syslog

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestSyslogInput(t *testing.T) {
	basicConfig := func() *syslog.SyslogParserConfig {
		cfg := syslog.NewSyslogParserConfig("test_syslog_parser")
		return cfg
	}

	cases, err := syslog.CreateCases(basicConfig)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(fmt.Sprintf("TCP-%s", tc.Name), func(t *testing.T) {
			SyslogInputTest(t, NewSyslogInputConfigWithTcp(&tc.Config.SyslogBaseConfig), tc)
		})
		t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
			SyslogInputTest(t, NewSyslogInputConfigWithUdp(&tc.Config.SyslogBaseConfig), tc)
		})
	}
}

func SyslogInputTest(t *testing.T, cfg *SyslogInputConfig, tc syslog.Case) {
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	ops := []operator.Operator{op, fake}
	p, err := pipeline.NewDirectedPipeline(ops)
	require.NoError(t, err)

	err = p.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)

	var conn net.Conn
	if cfg.Tcp != nil {
		conn, err = net.Dial("tcp", cfg.Tcp.ListenAddress)
		require.NoError(t, err)
	}
	if cfg.Udp != nil {
		conn, err = net.Dial("udp", cfg.Udp.ListenAddress)
		require.NoError(t, err)
	}

	if v, ok := tc.Input.Body.(string); ok {
		_, err = conn.Write([]byte(v))
	} else {
		_, err = conn.Write(tc.Input.Body.([]byte))
	}

	conn.Close()
	require.NoError(t, err)

	defer p.Stop()
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
	basicConfig := func() *syslog.SyslogBaseConfig {
		cfg := syslog.NewSyslogParserConfig("test_syslog_parser")
		cfg.Protocol = "RFC3164"
		return &cfg.SyslogBaseConfig
	}

	t.Run("TCP", func(t *testing.T) {
		cfg := NewSyslogInputConfigWithTcp(basicConfig())
		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)
		syslogInputOp := op.(*SyslogInput)
		require.Equal(t, "test_syslog_internal_tcp", syslogInputOp.tcp.ID())
		require.Equal(t, "test_syslog_internal_parser", syslogInputOp.parser.ID())
		require.Equal(t, []string{syslogInputOp.parser.ID()}, syslogInputOp.tcp.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.parser.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.GetOutputIDs())
	})
	t.Run("UDP", func(t *testing.T) {
		cfg := NewSyslogInputConfigWithUdp(basicConfig())
		op, err := cfg.Build(testutil.Logger(t))
		require.NoError(t, err)
		syslogInputOp := op.(*SyslogInput)
		require.Equal(t, "test_syslog_internal_udp", syslogInputOp.udp.ID())
		require.Equal(t, "test_syslog_internal_parser", syslogInputOp.parser.ID())
		require.Equal(t, []string{syslogInputOp.parser.ID()}, syslogInputOp.udp.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.parser.GetOutputIDs())
		require.Equal(t, []string{"fake"}, syslogInputOp.GetOutputIDs())
	})
}

func NewSyslogInputConfigWithTcp(syslogCfg *syslog.SyslogBaseConfig) *SyslogInputConfig {
	cfg := NewSyslogInputConfig("test_syslog")
	cfg.SyslogBaseConfig = *syslogCfg
	cfg.Tcp = &tcp.NewTCPInputConfig("test_syslog_tcp").TCPBaseConfig
	cfg.Tcp.ListenAddress = ":14201"
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func NewSyslogInputConfigWithUdp(syslogCfg *syslog.SyslogBaseConfig) *SyslogInputConfig {
	cfg := NewSyslogInputConfig("test_syslog")
	cfg.SyslogBaseConfig = *syslogCfg
	cfg.Udp = &udp.NewUDPInputConfig("test_syslog_udp").UDPBaseConfig
	cfg.Udp.ListenAddress = ":12032"
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func TestConfigYamlUnmarshalUDP(t *testing.T) {
	base := `type: syslog_input
protocol: rfc5424
udp:
  listen_address: localhost:1234
`
	var cfg SyslogInputConfig
	err := yaml.Unmarshal([]byte(base), &cfg)
	require.NoError(t, err)
	require.Equal(t, syslog.RFC5424, cfg.Protocol)
	require.Nil(t, cfg.Tcp)
	require.NotNil(t, cfg.Udp)
	require.Equal(t, "localhost:1234", cfg.Udp.ListenAddress)
}

func TestConfigYamlUnmarshalTCP(t *testing.T) {
	base := `type: syslog_input
protocol: rfc5424
tcp:
  listen_address: localhost:1234
  tls:
    ca_file: /tmp/test.ca
`
	var cfg SyslogInputConfig
	err := yaml.Unmarshal([]byte(base), &cfg)
	require.NoError(t, err)
	require.Equal(t, syslog.RFC5424, cfg.Protocol)
	require.Nil(t, cfg.Udp)
	require.NotNil(t, cfg.Tcp)
	require.Equal(t, "localhost:1234", cfg.Tcp.ListenAddress)
	require.Equal(t, "/tmp/test.ca", cfg.Tcp.TLS.CAFile)
}
