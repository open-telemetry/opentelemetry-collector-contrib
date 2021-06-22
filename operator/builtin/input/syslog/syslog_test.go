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

	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/tcp"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/udp"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/parser/syslog"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
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
			SyslogInputTest(t, NewSyslogInputConfigWithTcp(tc.Config), tc)
		})
		t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
			SyslogInputTest(t, NewSyslogInputConfigWithUdp(tc.Config), tc)
		})
	}
}

func SyslogInputTest(t *testing.T, cfg *SyslogInputConfig, tc syslog.Case) {
	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	ops = append(ops, fake)
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

	if v, ok := tc.InputBody.(string); ok {
		_, err = conn.Write([]byte(v))
	} else {
		_, err = conn.Write(tc.InputBody.([]byte))
	}

	conn.Close()
	require.NoError(t, err)

	defer p.Stop()
	select {
	case e := <-fake.Received:
		// close pipeline to avoid data race
		require.Equal(t, tc.ExpectedBody, e.Body)
		require.Equal(t, tc.ExpectedTimestamp, e.Timestamp)
		require.Equal(t, tc.ExpectedSeverity, e.Severity)
		require.Equal(t, tc.ExpectedSeverityText, e.SeverityText)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be processed")
	}
}

func TestSyslogIDs(t *testing.T) {
	basicConfig := func() *syslog.SyslogParserConfig {
		cfg := syslog.NewSyslogParserConfig("test_syslog_parser")
		return cfg
	}

	cases := []struct {
		Name          string
		Cfg           *syslog.SyslogParserConfig
		ExpectedOpIDs []string
		UDPorTCP      string
	}{
		{
			Name: "default",
			Cfg: func() *syslog.SyslogParserConfig {
				sysCfg := basicConfig()
				sysCfg.Protocol = "RFC3164"
				return sysCfg
			}(),
			UDPorTCP: "UDP",
			ExpectedOpIDs: []string{
				"$.test_syslog.test_syslog_parser",
				"$.fake",
			},
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("TCP-%s", tc.Name), func(t *testing.T) {
			cfg := NewSyslogInputConfigWithTcp(tc.Cfg)
			bc := testutil.NewBuildContext(t)
			ops, err := cfg.Build(bc)
			require.NoError(t, err)
			for i, op := range ops {
				out := op.GetOutputIDs()
				require.Equal(t, tc.ExpectedOpIDs[i], out[0])
			}
		})
		t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
			cfg := NewSyslogInputConfigWithUdp(tc.Cfg)
			bc := testutil.NewBuildContext(t)
			ops, err := cfg.Build(bc)
			require.NoError(t, err)
			for i, op := range ops {
				out := op.GetOutputIDs()
				require.Equal(t, tc.ExpectedOpIDs[i], out[0])
			}
		})
	}
}

func NewSyslogInputConfigWithTcp(syslogCfg *syslog.SyslogParserConfig) *SyslogInputConfig {
	cfg := NewSyslogInputConfig("test_syslog")
	cfg.SyslogParserConfig = *syslogCfg
	cfg.Tcp = tcp.NewTCPInputConfig("test_syslog_tcp")
	cfg.Tcp.ListenAddress = ":14201"
	cfg.OutputIDs = []string{"$.fake"}
	return cfg
}

func NewSyslogInputConfigWithUdp(syslogCfg *syslog.SyslogParserConfig) *SyslogInputConfig {
	cfg := NewSyslogInputConfig("test_syslog")
	cfg.SyslogParserConfig = *syslogCfg
	cfg.Udp = udp.NewUDPInputConfig("test_syslog_udp")
	cfg.Udp.ListenAddress = ":12032"
	cfg.OutputIDs = []string{"$.fake"}
	return cfg
}

func TestConfigYamlUnmarshal(t *testing.T) {
	base := `type: syslog_input
protocol: rfc5424
udp:
  listen_address: localhost:1234
`
	var cfg SyslogInputConfig
	err := yaml.Unmarshal([]byte(base), &cfg)
	require.NoError(t, err)
	require.Equal(t, syslog.RFC5424, cfg.Protocol)
	require.Equal(t, "localhost:1234", cfg.Udp.ListenAddress)

	base = `type: syslog_input
protocol: rfc5424
tcp:
  listen_address: localhost:1234
  tls:
    ca_file: /tmp/test.ca 
`
	err = yaml.Unmarshal([]byte(base), &cfg)
	require.NoError(t, err)
	require.Equal(t, syslog.RFC5424, cfg.Protocol)
	require.Equal(t, "localhost:1234", cfg.Tcp.ListenAddress)
	require.Equal(t, "/tmp/test.ca", cfg.Tcp.TLS.CAFile)
}
