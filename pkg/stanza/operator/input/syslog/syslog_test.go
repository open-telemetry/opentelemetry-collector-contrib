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

func TestInput(t *testing.T) {
	basicConfig := func() *syslog.ParserConfig {
		cfg := syslog.NewParserConfig("test_syslog_parser")
		return cfg
	}

	cases, err := syslog.CreateCases(basicConfig)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(fmt.Sprintf("TCP-%s", tc.Name), func(t *testing.T) {
			SyslogInputTest(t, NewInputConfigWithTCP(&tc.Config.BaseConfig), tc)
		})
		t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
			SyslogInputTest(t, NewInputConfigWithUDP(&tc.Config.BaseConfig), tc)
		})
	}
}

func SyslogInputTest(t *testing.T, cfg *InputConfig, tc syslog.Case) {
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	ops := []operator.Operator{op, fake}
	p, err := pipeline.NewDirectedPipeline(ops)
	require.NoError(t, err)

	err = p.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)

	var conn net.Conn
	if cfg.TCPConfig != nil {
		conn, err = net.Dial("tcp", cfg.TCPConfig.ListenAddress)
		require.NoError(t, err)
	}
	if cfg.UDPConfig != nil {
		conn, err = net.Dial("udp", cfg.UDPConfig.ListenAddress)
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
	basicConfig := func() *syslog.BaseConfig {
		cfg := syslog.NewParserConfig("test_syslog_parser")
		cfg.Protocol = "RFC3164"
		return &cfg.BaseConfig
	}

	t.Run("TCP", func(t *testing.T) {
		cfg := NewInputConfigWithTCP(basicConfig())
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
		cfg := NewInputConfigWithUDP(basicConfig())
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

func NewInputConfigWithTCP(syslogCfg *syslog.BaseConfig) *InputConfig {
	cfg := NewInputConfig("test_syslog")
	cfg.BaseConfig = *syslogCfg
	cfg.TCPConfig = &tcp.NewInputConfig("test_syslog_tcp").BaseConfig
	cfg.TCPConfig.ListenAddress = ":14201"
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func NewInputConfigWithUDP(syslogCfg *syslog.BaseConfig) *InputConfig {
	cfg := NewInputConfig("test_syslog")
	cfg.BaseConfig = *syslogCfg
	cfg.UDPConfig = &udp.NewInputConfig("test_syslog_udp").BaseConfig
	cfg.UDPConfig.ListenAddress = ":12032"
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func TestConfigYamlUnmarshalUDP(t *testing.T) {
	base := `type: syslog_input
protocol: rfc5424
udp:
  listen_address: localhost:1234
`
	var cfg InputConfig
	err := yaml.Unmarshal([]byte(base), &cfg)
	require.NoError(t, err)
	require.Equal(t, syslog.RFC5424, cfg.Protocol)
	require.Nil(t, cfg.TCPConfig)
	require.NotNil(t, cfg.UDPConfig)
	require.Equal(t, "localhost:1234", cfg.UDPConfig.ListenAddress)
}

func TestConfigYamlUnmarshalTCP(t *testing.T) {
	base := `type: syslog_input
protocol: rfc5424
tcp:
  listen_address: localhost:1234
  tls:
    ca_file: /tmp/test.ca 
`
	var cfg InputConfig
	err := yaml.Unmarshal([]byte(base), &cfg)
	require.NoError(t, err)
	require.Equal(t, syslog.RFC5424, cfg.Protocol)
	require.Nil(t, cfg.UDPConfig)
	require.NotNil(t, cfg.TCPConfig)
	require.Equal(t, "localhost:1234", cfg.TCPConfig.ListenAddress)
	require.Equal(t, "/tmp/test.ca", cfg.TCPConfig.TLS.CAFile)
}
