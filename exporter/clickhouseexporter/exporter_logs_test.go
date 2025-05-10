// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestLogsExporter_New(t *testing.T) {
	type validate func(*testing.T, *logsExporter, error)

	_ = func(t *testing.T, exporter *logsExporter, err error) {
		require.NoError(t, err)
		require.NotNil(t, exporter)
	}

	_ = func(want error) validate {
		return func(t *testing.T, exporter *logsExporter, err error) {
			require.Nil(t, exporter)
			require.Error(t, err)
			if !errors.Is(err, want) {
				t.Fatalf("Expected error '%v', but got '%v'", want, err)
			}
		}
	}

	failWithMsg := func(msg string) validate {
		return func(t *testing.T, _ *logsExporter, err error) {
			require.ErrorContains(t, err, msg)
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
	}{
		"no dsn": {
			config: withDefaultConfig(),
			want:   failWithMsg("exec create logs table sql: parse dsn address failed"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			var err error
			exporter, err := newLogsExporter(zap.NewNop(), test.config)
			err = errors.Join(err, err)

			if exporter != nil {
				err = errors.Join(err, exporter.start(context.TODO(), nil))
				defer func() {
					require.NoError(t, exporter.shutdown(context.TODO()))
				}()
			}

			test.want(t, exporter, err)
		})
	}
}

func TestExporter_pushLogsData(t *testing.T) {
	t.Run("push success", func(t *testing.T) {
		var items int
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			t.Logf("%d, values:%+v", items, values)
			if strings.HasPrefix(query, "INSERT") {
				items++
			}
			return nil
		})

		exporter := newTestLogsExporter(t, defaultEndpoint)
		mustPushLogsData(t, exporter, simpleLogs(1))
		mustPushLogsData(t, exporter, simpleLogs(2))

		require.Equal(t, 3, items)
	})
	t.Run("test check resource metadata", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				require.Equal(t, "https://opentelemetry.io/schemas/1.4.0", values[8])
				require.Equal(t, map[string]string{
					"service.name": "test-service",
				}, values[9])
			}
			return nil
		})
		exporter := newTestLogsExporter(t, defaultEndpoint)
		mustPushLogsData(t, exporter, simpleLogs(1))
	})
	t.Run("test check scope metadata", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				require.Equal(t, "https://opentelemetry.io/schemas/1.7.0", values[10])
				require.Equal(t, "io.opentelemetry.contrib.clickhouse", values[11])
				require.Equal(t, "1.0.0", values[12])
				require.Equal(t, map[string]string{
					"lib": "clickhouse",
				}, values[13])
			}
			return nil
		})
		exporter := newTestLogsExporter(t, defaultEndpoint)
		mustPushLogsData(t, exporter, simpleLogs(1))
	})
	t.Run("test with only observed timestamp", func(t *testing.T) {
		initClickhouseTestServer(t, func(query string, values []driver.Value) error {
			if strings.HasPrefix(query, "INSERT") {
				require.NotEqual(t, "0", values[0])
			}
			return nil
		})

		exporter := newTestLogsExporter(t, defaultEndpoint)
		mustPushLogsData(t, exporter, simpleLogsWithNoTimestamp(1))
	})
}

func TestLogsClusterConfig(t *testing.T) {
	testClusterConfig(t, func(t *testing.T, dsn string, clusterTest clusterTestConfig, fns ...func(*Config)) {
		exporter := newTestLogsExporter(t, dsn, fns...)
		clusterTest.verifyConfig(t, exporter.cfg)
	})
}

func TestLogsTableEngineConfig(t *testing.T) {
	testTableEngineConfig(t, func(t *testing.T, dsn string, engineTest tableEngineTestConfig, fns ...func(*Config)) {
		exporter := newTestLogsExporter(t, dsn, fns...)
		engineTest.verifyConfig(t, exporter.cfg.TableEngine)
	})
}

func newTestLogsExporter(t *testing.T, dsn string, fns ...func(*Config)) *logsExporter {
	exporter, err := newLogsExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))
	require.NoError(t, err)
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
}

func withTestExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(endpoint string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.Endpoint = endpoint
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}

func simpleLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	sl.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("lib", "clickhouse")
	timestamp := time.Unix(1703498029, 0)
	for i := 0; i < count; i++ {
		r := sl.LogRecords().AppendEmpty()
		r.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.Body().SetStr("error message")
		r.Attributes().PutStr(conventions.AttributeServiceNamespace, "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}

func simpleLogsWithNoTimestamp(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.4.0")
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.7.0")
	sl.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("lib", "clickhouse")
	timestamp := time.Unix(1703498029, 0)
	for i := 0; i < count; i++ {
		r := sl.LogRecords().AppendEmpty()
		r.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp))
		r.SetSeverityNumber(plog.SeverityNumberError2)
		r.SetSeverityText("error")
		r.Body().SetStr("error message")
		r.Attributes().PutStr(conventions.AttributeServiceNamespace, "default")
		r.SetFlags(plog.DefaultLogRecordFlags)
		r.SetTraceID([16]byte{1, 2, 3, byte(i)})
		r.SetSpanID([8]byte{1, 2, 3, byte(i)})
	}
	return logs
}

func mustPushLogsData(t *testing.T, exporter *logsExporter, ld plog.Logs) {
	err := exporter.pushLogsData(context.TODO(), ld)
	require.NoError(t, err)
}

func initClickhouseTestServer(t *testing.T, recorder recorder) {
	driverName = t.Name()
	sql.Register(t.Name(), &testClickhouseDriver{
		recorder: recorder,
	})
}

type recorder func(query string, values []driver.Value) error

type testClickhouseDriver struct {
	recorder recorder
}

func (t *testClickhouseDriver) Open(_ string) (driver.Conn, error) {
	return &testClickhouseDriverConn{
		recorder: t.recorder,
	}, nil
}

type testClickhouseDriverConn struct {
	recorder recorder
}

func (t *testClickhouseDriverConn) Prepare(query string) (driver.Stmt, error) {
	return &testClickhouseDriverStmt{
		query:    query,
		recorder: t.recorder,
	}, nil
}

func (*testClickhouseDriverConn) Close() error {
	return nil
}

func (*testClickhouseDriverConn) Begin() (driver.Tx, error) {
	return &testClickhouseDriverTx{}, nil
}

func (*testClickhouseDriverConn) CheckNamedValue(_ *driver.NamedValue) error {
	return nil
}

type testClickhouseDriverStmt struct {
	query    string
	recorder recorder
}

func (*testClickhouseDriverStmt) Close() error {
	return nil
}

func (t *testClickhouseDriverStmt) NumInput() int {
	return strings.Count(t.query, "?")
}

func (t *testClickhouseDriverStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, t.recorder(t.query, args)
}

func (t *testClickhouseDriverStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return nil, nil
}

type testClickhouseDriverTx struct {
}

func (*testClickhouseDriverTx) Commit() error {
	return nil
}

func (*testClickhouseDriverTx) Rollback() error {
	return nil
}
