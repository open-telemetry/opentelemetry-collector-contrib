// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
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

// TestSyslogParserDoesNotSplitBatches verifies that the syslog parser processes
// batches of entries without splitting them
func TestSyslogParserDoesNotSplitBatches(t *testing.T) {
	output := &testutil.Operator{}
	output.On("ID").Return("test-output")
	output.On("CanProcess").Return(true)
	output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)

	cfg := basicConfig()
	cfg.Protocol = syslog.RFC5424
	cfg.OutputIDs = []string{"test-output"}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	err = op.SetOutputs([]operator.Operator{output})
	require.NoError(t, err)

	ctx := t.Context()

	// Create test entries with valid syslog messages
	entry1 := entry.New()
	entry1.Body = "<34>1 2003-10-11T22:14:15.003Z mymachine.example.com su - ID47 - 'su root' failed"

	entry2 := entry.New()
	entry2.Body = "<165>1 2003-08-24T05:14:15.000003-07:00 192.0.2.1 myproc 8710 - - %% It's time to make the do-nuts."

	entry3 := entry.New()
	entry3.Body = "<13>1 2003-10-11T22:14:15.003Z mymachine.example.com evntslog - ID47 - An application event log entry"

	testEntries := []*entry.Entry{entry1, entry2, entry3}

	err = op.ProcessBatch(ctx, testEntries)
	require.NoError(t, err)

	// Verify that ProcessBatch was called exactly once with the batch of entries
	// This proves that the batch was not split into individual entries
	output.AssertCalled(t, "ProcessBatch", ctx, mock.MatchedBy(func(entries []*entry.Entry) bool {
		return len(entries) == 3
	}))
	output.AssertNumberOfCalls(t, "ProcessBatch", 1)
}

func TestSyslogQuietModeProcessBatch(t *testing.T) {
	testCases := []struct {
		name        string
		onError     string
		expectError bool
	}{
		{
			name:        "DropOnErrorQuiet_ReturnsNoError",
			onError:     "drop_quiet",
			expectError: false,
		},
		{
			name:        "SendOnErrorQuiet_ReturnsNoError",
			onError:     "send_quiet",
			expectError: false,
		},
		{
			name:        "DropOnError_ReturnsError",
			onError:     "drop",
			expectError: true,
		},
		{
			name:        "SendOnError_ReturnsError",
			onError:     "send",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := basicConfig()
			cfg.Protocol = syslog.RFC5424
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			output := &testutil.Operator{}
			output.On("ID").Return("fake")
			output.On("CanProcess").Return(true)
			output.On("ProcessBatch", mock.Anything, mock.Anything).Return(nil)
			require.NoError(t, op.SetOutputs([]operator.Operator{output}))

			ctx := t.Context()

			// Create entries with invalid syslog messages that will cause parse errors
			entry1 := entry.New()
			entry1.Body = "invalid syslog message 1"

			entry2 := entry.New()
			entry2.Body = "invalid syslog message 2"

			entry3 := entry.New()
			entry3.Body = "invalid syslog message 3"

			testEntries := []*entry.Entry{entry1, entry2, entry3}

			err = op.ProcessBatch(ctx, testEntries)
			if tc.expectError {
				require.Error(t, err, "expected error in non-quiet mode")
			} else {
				require.NoError(t, err, "expected no error in quiet mode")
			}
		})
	}
}

// TestSyslogQuietModeProcess tests quiet mode handling for toBytes error
// in the Process method when allowSkipPriHeader is enabled
func TestSyslogQuietModeProcess(t *testing.T) {
	testCases := []struct {
		name        string
		onError     string
		expectError bool
	}{
		{
			name:        "DropOnErrorQuiet_ReturnsNoError",
			onError:     "drop_quiet",
			expectError: false,
		},
		{
			name:        "SendOnErrorQuiet_ReturnsNoError",
			onError:     "send_quiet",
			expectError: false,
		},
		{
			name:        "DropOnError_ReturnsError",
			onError:     "drop",
			expectError: true,
		},
		{
			name:        "SendOnError_ReturnsError",
			onError:     "send",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := basicConfig()
			cfg.Protocol = syslog.RFC3164
			cfg.AllowSkipPriHeader = true // Enable this to trigger the toBytes path
			cfg.OnError = tc.onError

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			output := &testutil.Operator{}
			output.On("ID").Return("fake")
			output.On("CanProcess").Return(true)
			output.On("Process", mock.Anything, mock.Anything).Return(nil)
			require.NoError(t, op.SetOutputs([]operator.Operator{output}))

			// Create entry with non-string body that will cause toBytes error
			e := entry.New()
			e.Body = 12345 // Non-string body causes toBytes to fail
			e.ObservedTimestamp = time.Now()

			err = op.Process(t.Context(), e)
			if tc.expectError {
				require.Error(t, err, "expected error in non-quiet mode")
			} else {
				require.NoError(t, err, "expected no error in quiet mode")
			}
		})
	}
}
