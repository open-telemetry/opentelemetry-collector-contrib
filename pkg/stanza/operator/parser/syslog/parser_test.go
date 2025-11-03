// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog/syslogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func basicConfig() *syslog.Config {
	cfg := syslog.NewConfigWithID("test_operator_id")
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func TestParser(t *testing.T) {
	cases, err := syslogtest.CreateCases(basicConfig)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			set := componenttest.NewNopTelemetrySettings()
			op, err := tc.Config.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			err = op.SetOutputs([]operator.Operator{fake})
			require.NoError(t, err)

			newEntry := tc.Input
			ots := newEntry.ObservedTimestamp

			err = op.Process(t.Context(), newEntry)
			require.NoError(t, err)

			select {
			case e := <-fake.Received:
				require.Equal(t, ots, e.ObservedTimestamp)
				require.Equal(t, tc.Expect, newEntry)
			case <-time.After(time.Second):
				require.FailNow(t, "Timed out waiting for entry to be processed")
			}
		})
	}
}

func TestSyslogParseRFC5424_SDNameTooLong(t *testing.T) {
	cfg := basicConfig()
	cfg.Protocol = syslog.RFC5424

	body := `<86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [verylongsdnamethatisgreaterthan32bytes@12345 UserHostAddress="192.168.2.132"] my message`

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	err = op.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	newEntry := entry.New()
	newEntry.Body = body
	err = op.Process(t.Context(), newEntry)
	require.ErrorContains(t, err, "expecting a structured data element id (from 1 to max 32 US-ASCII characters")

	select {
	case e := <-fake.Received:
		require.Equal(t, body, e.Body)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be processed")
	}
}

func TestSyslogParseRFC5424_Octet_Counting_MessageTooLong(t *testing.T) {
	cfg := basicConfig()
	cfg.Protocol = syslog.RFC5424
	cfg.EnableOctetCounting = true
	cfg.MaxOctets = 214

	body := `215 <86>1 2015-08-05T21:58:59.693Z 192.168.2.132 SecureAuth0 23108 ID52020 [SecureAuth@27389 UserHostAddress="192.168.2.132" Realm="SecureAuth0" UserID="Tester2" PEN="27389"] Found the user for retrieving user's profile`

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	err = op.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	newEntry := entry.New()
	newEntry.Body = body
	err = op.Process(t.Context(), newEntry)
	require.ErrorContains(t, err, "message length (215) exceeds maximum length (214)")

	select {
	case e := <-fake.Received:
		require.Equal(t, body, e.Body)
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for entry to be processed")
	}
}

func TestSyslogProtocolConfig(t *testing.T) {
	for _, proto := range []string{"RFC5424", "rfc5424", "RFC3164", "rfc3164"} {
		cfg := basicConfig()
		cfg.Protocol = proto
		set := componenttest.NewNopTelemetrySettings()
		_, err := cfg.Build(set)
		require.NoError(t, err)
	}

	for _, proto := range []string{"RFC5424a", "rfc5424b", "RFC3164c", "rfc3164d"} {
		cfg := basicConfig()
		cfg.Protocol = proto
		set := componenttest.NewNopTelemetrySettings()
		_, err := cfg.Build(set)
		require.Error(t, err)
	}
}
