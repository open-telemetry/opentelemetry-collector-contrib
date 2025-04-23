// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog/syslogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

var (
	ts          = time.Now()
	basicConfig = func() *syslog.Config {
		cfg := syslog.NewConfigWithID("test_syslog_parser")
		return cfg
	}
	OctetCase = syslogtest.Case{
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
	WithMetadata = syslogtest.Case{
		Name: "RFC3164",
		Config: func() *syslog.Config {
			cfg := basicConfig()
			cfg.Protocol = syslog.RFC3164
			return cfg
		}(),
		Input: &entry.Entry{
			Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
		},
		Expect: &entry.Entry{
			Timestamp:    time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, time.UTC),
			Severity:     entry.Error2,
			SeverityText: "crit",
			Resource: map[string]any{
				"service.name": "apache_server",
			},
			Attributes: map[string]any{
				"foo":      "bar",
				"appname":  "apache_server",
				"facility": 4,
				"hostname": "1.2.3.4",
				"message":  "test message",
				"priority": 34,
			},
			Body: fmt.Sprintf("<34>%s 1.2.3.4 apache_server: test message", ts.Format("Jan _2 15:04:05")),
		},
		ValidForTCP: true,
		ValidForUDP: true,
	}
)

func TestInput(t *testing.T) {
	cases, err := syslogtest.CreateCases(basicConfig)
	require.NoError(t, err)
	cases = append(cases, OctetCase)

	for _, tc := range cases {
		cfg := tc.Config.BaseConfig
		if tc.ValidForTCP {
			tcpCfg := NewConfigWithTCP(&cfg)
			if tc.Name == syslogtest.RFC6587OctetCountingPreserveSpaceTest {
				tcpCfg.TCP.TrimConfig.PreserveLeading = true
				tcpCfg.TCP.TrimConfig.PreserveTrailing = true
			}
			t.Run(fmt.Sprintf("TCP-%s", tc.Name), func(t *testing.T) {
				InputTest(t, tc, tcpCfg, nil, nil)
			})
		}
		if tc.ValidForUDP {
			udpCfg := NewConfigWithUDP(&cfg)
			if tc.Name == syslogtest.RFC6587OctetCountingPreserveSpaceTest {
				udpCfg.UDP.TrimConfig.PreserveLeading = true
				udpCfg.UDP.TrimConfig.PreserveTrailing = true
			}
			t.Run(fmt.Sprintf("UDP-%s", tc.Name), func(t *testing.T) {
				InputTest(t, tc, udpCfg, nil, nil)
			})
		}
	}

	withMetadataCfg := WithMetadata.Config.BaseConfig
	t.Run("TCPWithMetadata", func(t *testing.T) {
		cfg := NewConfigWithTCP(&withMetadataCfg)
		cfg.IdentifierConfig = helper.NewIdentifierConfig()
		cfg.Resource["service.name"] = helper.ExprStringConfig("apache_server")
		cfg.AttributerConfig = helper.NewAttributerConfig()
		cfg.Attributes["foo"] = helper.ExprStringConfig("bar")
		InputTest(t, WithMetadata, cfg, map[string]any{"service.name": "apache_server"}, map[string]any{"foo": "bar"})
	})

	t.Run("UDPWithMetadata", func(t *testing.T) {
		cfg := NewConfigWithUDP(&withMetadataCfg)
		cfg.IdentifierConfig = helper.NewIdentifierConfig()
		cfg.Resource["service.name"] = helper.ExprStringConfig("apache_server")
		cfg.AttributerConfig = helper.NewAttributerConfig()
		cfg.Attributes["foo"] = helper.ExprStringConfig("bar")
		InputTest(t, WithMetadata, cfg, map[string]any{"service.name": "apache_server"}, map[string]any{"foo": "bar"})
	})
}

func InputTest(t *testing.T, tc syslogtest.Case, cfg *Config, rsrc map[string]any, attr map[string]any) {
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
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

		expect := tc.Expect
		expect.ObservedTimestamp = ots
		if rsrc != nil {
			if expect.Resource == nil {
				expect.Resource = rsrc
			} else {
				for k, v := range rsrc {
					expect.Resource[k] = v
				}
			}
		}
		if attr != nil {
			if expect.Attributes == nil {
				expect.Attributes = attr
			} else {
				for k, v := range attr {
					expect.Attributes[k] = v
				}
			}
		}
		require.Equal(t, expect, e)
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
		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
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
		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
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
			name:  "OneLogTrailingSpace",
			input: []byte(`84 <13>1 2024-02-28T03:32:00.313226+00:00 192.168.65.1 inactive - - -  partition is p2 `),
			steps: []splittest.Step{
				splittest.ExpectToken(`84 <13>1 2024-02-28T03:32:00.313226+00:00 192.168.65.1 inactive - - -  partition is p2 `),
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
