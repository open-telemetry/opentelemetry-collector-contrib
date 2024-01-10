// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

var (
	basicConfig = func() *syslog.Config {
		cfg := syslog.NewConfigWithID("test_syslog_parser")
		return cfg
	}
	OctetCase = syslog.Case{
		Name: "RFC6587 Octet Counting",
		Config: func() *syslog.Config {
			cfg := basicConfig()
			cfg.Protocol = syslog.RFC5424
			cfg.EnableOctetCounting = true
			return cfg
		}(),
		Input: &entry.Entry{
			Body: `215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile215 <86>1 2016-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile215 <86>1 2017-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
		},
		Expect: &entry.Entry{
			Timestamp:    time.Date(2015, 8, 5, 21, 58, 59, 693000000, time.UTC),
			Severity:     entry.Info,
			SeverityText: "info",
			Attributes: map[string]any{
				"appname":  "SecureAuth0",
				"facility": 10,
				"hostname": "192.168.2.132",
				"message":  "Found the user for retrieving user's profile",
				"msg_id":   "ID52020",
				"priority": 86,
				"proc_id":  "23108",
				"structured_data": map[string]any{
					"SecureAuth@27389": map[string]any{
						"PEN":             "27389",
						"Realm":           "SecureAuth0",
						"UserHostAddress": "192.168.2.132",
						"UserID":          "Tester2",
					},
				},
				"version": 1,
			},
			Body: `215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`,
		},
		ValidForTCP: true,
	}
)

func TestInput(t *testing.T) {

	cases, err := syslog.CreateCases(basicConfig)
	require.NoError(t, err)
	cases = append(cases, OctetCase)

	for _, tc := range cases {
		cfg := tc.Config.BaseConfig
		if tc.ValidForTCP {
			t.Run(fmt.Sprintf("TCP-%s", tc.Name), func(t *testing.T) {
				InputTest(t, NewConfigWithTCP(&cfg), tc)
			})
		}

		if tc.ValidForUDP {
			t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
				InputTest(t, NewConfigWithUDP(&cfg), tc)
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

	err = p.Start(testutil.NewUnscopedMockPersister())
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

func TestOctetFramingSplitFunc(t *testing.T) {
	testCases := []struct {
		name  string
		input []byte
		steps []splittest.Step
	}{
		{
			name:  "OneLogSimple",
			input: []byte(`17 my log LOGEND 123`),
			steps: []splittest.Step{
				splittest.ExpectToken(`17 my log LOGEND 123`),
			},
		},
		{
			name:  "TwoLogsSimple",
			input: []byte(`17 my log LOGEND 12317 my log LOGEND 123`),
			steps: []splittest.Step{
				splittest.ExpectToken(`17 my log LOGEND 123`),
				splittest.ExpectToken(`17 my log LOGEND 123`),
			},
		},
		{
			name:  "NoMatches",
			input: []byte(`no matches in it`),
			steps: []splittest.Step{
				splittest.ExpectToken(`no matches in it`),
			},
		},
		{
			name:  "NonMatchesAfter",
			input: []byte(`17 my log LOGEND 123my log LOGEND 12317 my log LOGEND 123`),
			steps: []splittest.Step{
				splittest.ExpectToken(`17 my log LOGEND 123`),
				splittest.ExpectToken(`my log LOGEND 12317 my log LOGEND 123`),
			},
		},
		{
			name: "HugeLog10000",
			input: func() []byte {
				newRaw := splittest.GenerateBytes(10000)
				newRaw = append([]byte(`10000 `), newRaw...)
				return newRaw
			}(),
			steps: []splittest.Step{
				splittest.ExpectToken(`10000 ` + string(splittest.GenerateBytes(10000))),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc, err := OctetSplitFuncBuilder(nil)
		require.NoError(t, err)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}
