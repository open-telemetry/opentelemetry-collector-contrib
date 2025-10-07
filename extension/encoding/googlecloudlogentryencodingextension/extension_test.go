// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension

import (
	"bytes"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func newTestExtension(t *testing.T, cfg Config) *ext {
	extension := newExtension(&cfg)
	err := extension.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		err = extension.Shutdown(t.Context())
		require.NoError(t, err)
	})
	return extension
}

func TestHandleLogLine(t *testing.T) {
	tests := []struct {
		name        string
		logLine     []byte
		expectedErr string
	}{
		{
			name:        "invalid log entry",
			logLine:     []byte("invalid"),
			expectedErr: `failed to unmarshal log entry`,
		},
		{
			name:        "invalid log entry fields",
			logLine:     []byte(`{"logName": "invalid"}`),
			expectedErr: `failed to handle log entry`,
		},
		{
			name: "valid",
			logLine: []byte(`{
	"logName": "projects/open-telemetry/logs/log-test", 
	"timestamp": "2024-05-05T10:31:19.45570687Z"
}`),
		},
	}

	extension := newTestExtension(t, Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logs := plog.NewLogs()
			err := extension.handleLogLine(logs, tt.logLine)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestUnmarshalLogs(t *testing.T) {
	// this test will test all common log fields at once
	data, err := os.ReadFile("testdata/log_entry.json")
	require.NoError(t, err)

	compacted := bytes.NewBuffer([]byte{})
	err = gojson.Compact(compacted, data)
	require.NoError(t, err)

	tests := []struct {
		name  string
		nLogs int
	}{
		{
			name:  "1 log",
			nLogs: 1,
		},
		{
			name:  "4 logs",
			nLogs: 4,
		},
	}

	extension := newTestExtension(t, Config{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payload with as many logs as defined in nLogs.
			// Each log takes up one line. A new line means a new log.
			buff := bytes.NewBuffer([]byte{})
			for i := 0; i < tt.nLogs; i++ {
				buff.Write(compacted.Bytes())
				buff.Write([]byte{'\n'})
			}

			logs, err := extension.UnmarshalLogs(buff.Bytes())
			require.NoError(t, err)

			expected, err := golden.ReadLogs("testdata/log_entry_expected.yaml")
			require.NoError(t, err)
			require.Equal(t, 1, expected.ResourceLogs().Len())
			require.Equal(t, 1, expected.ResourceLogs().At(0).ScopeLogs().Len())

			// expected logs is only for one log entry, so multiply by as
			// mine as input logs
			expectedLogs := plog.NewLogs()
			for i := 0; i < tt.nLogs; i++ {
				rl := expectedLogs.ResourceLogs().AppendEmpty()
				expected.ResourceLogs().At(0).Resource().CopyTo(rl.Resource())
				sl := rl.ScopeLogs()
				expected.ResourceLogs().At(0).ScopeLogs().CopyTo(sl)
			}

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}

func TestPayloads(t *testing.T) {
	// TODO Keep adding tests when adding new log types support

	tests := []struct {
		name             string
		logFilename      string
		expectedFilename string
		expectedErr      string
	}{
		{
			name:             "audit log - activity",
			logFilename:      "testdata/auditlog/activity.json",
			expectedFilename: "testdata/auditlog/activity_expected.yaml",
		},
		{
			name:             "audit log - data access",
			logFilename:      "testdata/auditlog/data_access.json",
			expectedFilename: "testdata/auditlog/data_access_expected.yaml",
		},
		{
			name:             "audit log - policy",
			logFilename:      "testdata/auditlog/policy.json",
			expectedFilename: "testdata/auditlog/policy_expected.yaml",
		},
		{
			name:             "audit log - system event",
			logFilename:      "testdata/auditlog/system_event.json",
			expectedFilename: "testdata/auditlog/system_event_expected.yaml",
		},
		{
			name:             "vpc flow log - 0 bytes sent",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-0-bytes-sent.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-0-bytes-sent_expected.yaml",
		},
		{
			name:             "vpc flow log - 19kb sent",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-19kb-sent.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-19kb-sent_expected.yaml",
		},
		{
			name:             "vpc flow log - 800 bytes sent",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-800-bytes-sent.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-800-bytes-sent_expected.yaml",
		},
		{
			name:             "vpc flow log - from compute engine",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-from-computeengine.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-from-computeengine_expected.yaml",
		},
		{
			name:             "vpc flow log - with dest vpc",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-w-dest-vpc.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-w-dest-vpc_expected.yaml",
		},
		{
			name:             "vpc flow log - with internet routing details",
			logFilename:      "testdata/vpc-flow-log/vpc-flow-log-w-internet-routing-details.json",
			expectedFilename: "testdata/vpc-flow-log/vpc-flow-log-w-internet-routing-details_expected.yaml",
		},
	}

	extension := newTestExtension(t, Config{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(tt.logFilename)
			require.NoError(t, err)

			content := bytes.NewBuffer([]byte{})
			err = gojson.Compact(content, data)
			require.NoError(t, err)

			logs, err := extension.UnmarshalLogs(content.Bytes())
			require.NoError(t, err)

			// write expected log with:
			// golden.WriteLogs(t, tt.expectedFilename, logs)

			expectedLogs, err := golden.ReadLogs(tt.expectedFilename)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs))
		})
	}
}
