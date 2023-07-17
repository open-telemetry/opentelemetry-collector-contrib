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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
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
			Attributes: map[string]interface{}{
				"appname":  "SecureAuth0",
				"facility": 10,
				"hostname": "192.168.2.132",
				"message":  "Found the user for retrieving user's profile",
				"msg_id":   "ID52020",
				"priority": 86,
				"proc_id":  "23108",
				"structured_data": map[string]map[string]string{
					"SecureAuth@27389": {
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

func TestOctetFramingSplitFunc(t *testing.T) {
	testCases := []internal.TokenizerTestCase{
		{
			Name: "OneLogSimple",
			Raw:  []byte(`17 my log LOGEND 123`),
			ExpectedTokenized: []string{
				`17 my log LOGEND 123`,
			},
		},
		{
			Name: "TwoLogsSimple",
			Raw:  []byte(`17 my log LOGEND 12317 my log LOGEND 123`),
			ExpectedTokenized: []string{
				`17 my log LOGEND 123`,
				`17 my log LOGEND 123`,
			},
		},
		{
			Name: "NoMatches",
			Raw:  []byte(`no matches in it`),
			ExpectedTokenized: []string{
				`no matches in it`,
			},
		},
		{
			Name: "NonMatchesAfter",
			Raw:  []byte(`17 my log LOGEND 123my log LOGEND 12317 my log LOGEND 123`),
			ExpectedTokenized: []string{
				`17 my log LOGEND 123`,
				`my log LOGEND 12317 my log LOGEND 123`,
			},
		},
		{
			Name: "HugeLog100",
			Raw: func() []byte {
				newRaw := internal.GeneratedByteSliceOfLength(100)
				newRaw = append([]byte(`100 `), newRaw...)
				return newRaw
			}(),
			ExpectedTokenized: []string{
				`100 ` + string(internal.GeneratedByteSliceOfLength(100)),
			},
		},
		{
			Name: "maxCapacity",
			Raw: func() []byte {
				newRaw := internal.GeneratedByteSliceOfLength(4091)
				newRaw = append([]byte(`4091 `), newRaw...)
				return newRaw
			}(),
			ExpectedTokenized: []string{
				`4091 ` + string(internal.GeneratedByteSliceOfLength(4091)),
			},
		},
		{
			Name: "over capacity",
			Raw: func() []byte {
				newRaw := internal.GeneratedByteSliceOfLength(4092)
				newRaw = append([]byte(`5000 `), newRaw...)
				return newRaw
			}(),
			ExpectedTokenized: []string{
				`5000 ` + string(internal.GeneratedByteSliceOfLength(4091)),
				`j`,
			},
		},
	}
	for _, tc := range testCases {
		splitFunc, err := OctetMultiLineBuilder(helper.Encoding{})
		require.NoError(t, err)
		t.Run(tc.Name, tc.RunFunc(splitFunc))
	}
}
