package syslog

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/tcp"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/input/udp"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/parser/syslog"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
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

	err = p.Start()
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


	switch tc.InputRecord.(type) {
	case string:
		_, err = conn.Write([]byte(tc.InputRecord.(string)))
	case []byte:
		_, err = conn.Write(tc.InputRecord.([]byte))
	}

	conn.Close()
	require.NoError(t, err)

	defer p.Stop()
	select {
	case e := <-fake.Received:
		// close pipeline to avoid data race
		require.Equal(t, tc.ExpectedRecord, e.Record)
		require.Equal(t, tc.ExpectedTimestamp, e.Timestamp)
		require.Equal(t, tc.ExpectedSeverity, e.Severity)
		require.Equal(t, tc.ExpectedSeverityText, e.SeverityText)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be processed")
	}
}

func NewSyslogInputConfigWithTcp(syslogCfg *syslog.SyslogParserConfig) *SyslogInputConfig {
	cfg := NewSyslogInputConfig("test_syslog")
	cfg.Tcp = tcp.NewTCPInputConfig("test_syslog_tcp")
	cfg.Tcp.ListenAddress = ":14201"
	cfg.OutputIDs = []string{"fake"}
	cfg.Syslog = syslogCfg
	return cfg
}

func NewSyslogInputConfigWithUdp(syslogCfg *syslog.SyslogParserConfig) *SyslogInputConfig {
	cfg := NewSyslogInputConfig("test_syslog")
	cfg.Udp = udp.NewUDPInputConfig("test_syslog_udp")
	cfg.Udp.ListenAddress = ":12032"
	cfg.OutputIDs = []string{"fake"}
	cfg.Syslog = syslogCfg
	return cfg
}
