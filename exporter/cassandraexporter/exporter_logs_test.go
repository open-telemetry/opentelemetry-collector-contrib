// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestNewCluster(t *testing.T) {
	testCases := map[string]struct {
		cfg                   *Config
		expectedAuthenticator gocql.Authenticator
		expectedErr           error
	}{
		"empty_auth": {
			cfg:                   withDefaultConfig(),
			expectedAuthenticator: nil,
		},
		"empty_username": {
			cfg: withDefaultConfig(func(config *Config) {
				config.Auth.Password = "pass"
			}),
			expectedAuthenticator: nil,
			expectedErr:           errors.New("empty auth.username"),
		},
		"empty_password": {
			cfg: withDefaultConfig(func(config *Config) {
				config.Auth.UserName = "user"
			}),
			expectedAuthenticator: nil,
			expectedErr:           errors.New("empty auth.password"),
		},
		"success_auth": {
			cfg: withDefaultConfig(func(config *Config) {
				config.Auth.UserName = "user"
				config.Auth.Password = "pass"
			}),
			expectedAuthenticator: gocql.PasswordAuthenticator{
				Username: "user",
				Password: "pass",
			},
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			c, err := newCluster(test.cfg)
			if err == nil {
				require.Equal(t, test.expectedAuthenticator, c.Authenticator)
			}
			require.Equal(t, test.expectedErr, err)
		})
	}
}

func TestLogSchemaKeepsDistinctRecords(t *testing.T) {
	cfg := withDefaultConfig()
	table := parseCreateLogTableSQL(cfg)
	require.Contains(t, table, `PRIMARY KEY ((partition_key), ts, id)`)
	require.NotContains(t, table, "PRIMARY KEY (SpanId, SeverityNumber)")
	require.Contains(t, parseInsertLogSQL(cfg), "partition_key")
}

func TestPushLogsDataDoesNotCollapseRecordsAndReturnsError(t *testing.T) {
	cfg := withDefaultConfig()
	fake := &fakeExecutor{}
	exp := &logsExporter{cfg: cfg, logger: zap.NewNop(), exec: fake}

	logs := testLogs()
	require.NoError(t, exp.pushLogsData(context.Background(), logs))
	require.Len(t, fake.batches, 2)

	traced := fake.batches[0]
	require.Len(t, traced, 2)
	require.NotEqual(t, traced[0][0], traced[1][0])
	require.Equal(t, traced[0][2], traced[0][3])

	untraced := fake.batches[1]
	require.Len(t, untraced, 1)
	partition, ok := untraced[0][2].(string)
	require.True(t, ok)
	require.Contains(t, partition, "untraced/")
	require.Empty(t, untraced[0][3])

	again := &fakeExecutor{}
	exp.exec = again
	require.NoError(t, exp.pushLogsData(context.Background(), logs))
	require.Equal(t, fake.batches[0][0][0], again.batches[0][0][0])
	require.Equal(t, fake.batches[0][1][0], again.batches[0][1][0])

	fake.err = errors.New("unavailable")
	exp.exec = fake
	err := exp.pushLogsData(context.Background(), logs)
	require.ErrorContains(t, err, "unavailable")
}

func TestLogRecordIDStableForSameInput(t *testing.T) {
	ts := time.Unix(10, 20).UTC()
	attrs := map[string]string{"b": "2", "a": "1"}
	first := logRecordID(0, "trace", "trace", "span", ts, 1, "INFO", 9, `"hello"`, attrs, attrs)
	second := logRecordID(0, "trace", "trace", "span", ts, 1, "INFO", 9, `"hello"`, map[string]string{"a": "1", "b": "2"}, attrs)
	other := logRecordID(1, "trace", "trace", "span", ts, 1, "INFO", 9, `"hello"`, attrs, attrs)
	require.Equal(t, first, second)
	require.NotEqual(t, first, other)
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}

func testLogs() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "api")
	records := rl.ScopeLogs().AppendEmpty().LogRecords()
	ts := time.Unix(50, 0).UTC()
	traceID := pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	first := records.AppendEmpty()
	first.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	first.SetTraceID(traceID)
	first.SetSpanID(pcommon.SpanID{1})
	first.SetSeverityNumber(plog.SeverityNumberInfo)
	first.SetSeverityText("INFO")
	first.Body().SetStr("one")

	second := records.AppendEmpty()
	second.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	second.SetTraceID(traceID)
	second.SetSpanID(pcommon.SpanID{1})
	second.SetSeverityNumber(plog.SeverityNumberInfo)
	second.SetSeverityText("INFO")
	second.Body().SetStr("two")

	untraced := records.AppendEmpty()
	untraced.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	untraced.SetSeverityNumber(plog.SeverityNumberError)
	untraced.Body().SetStr("no-trace")
	return logs
}

type fakeExecutor struct {
	batches [][][]any
	err     error
}

func (f *fakeExecutor) Execute(_ context.Context, _ string, rows [][]any) error {
	if f.err != nil {
		return f.err
	}
	copied := make([][]any, len(rows))
	for i := range rows {
		copied[i] = append([]any(nil), rows[i]...)
	}
	f.batches = append(f.batches, copied)
	return nil
}
