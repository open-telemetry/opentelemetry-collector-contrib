// Copyright 2020, OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestExporter_New(t *testing.T) {
	type validate func(*testing.T, *clickhouseExporter, error)

	_ = func(t *testing.T, exporter *clickhouseExporter, err error) {
		require.Nil(t, err)
		require.NotNil(t, exporter)
	}

	failWith := func(want error) validate {
		return func(t *testing.T, exporter *clickhouseExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			if !errors.Is(err, want) {
				t.Fatalf("Expected error '%v', but got '%v'", want, err)
			}
		}
	}

	_ = func(msg string) validate {
		return func(t *testing.T, exporter *clickhouseExporter, err error) {
			require.Nil(t, exporter)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), msg)
		}
	}

	tests := map[string]struct {
		config *Config
		want   validate
	}{
		"no dsn": {
			config: withDefaultConfig(),
			want:   failWith(errConfigNoDSN),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {

			exporter, err := newExporter(zap.NewNop(), test.config)
			if exporter != nil {
				defer func() {
					require.NoError(t, exporter.Shutdown(context.TODO()))
				}()
			}

			test.want(t, exporter, err)
		})
	}
}

const (
	defaultDSN = "tcp://127.0.0.1:9000?database=default"
)

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

		exporter := newTestExporter(t, defaultDSN)
		mustPushLogsData(t, exporter, simpleLogs(1))
		mustPushLogsData(t, exporter, simpleLogs(2))

		require.Equal(t, 3, items)
	})
}

func newTestExporter(t *testing.T, dsn string, fns ...func(*Config)) *clickhouseExporter {
	exporter, err := newExporter(zaptest.NewLogger(t), withTestExporterConfig(fns...)(dsn))
	require.NoError(t, err)

	t.Cleanup(func() { _ = exporter.Shutdown(context.TODO()) })
	return exporter
}

func withTestExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(dsn string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.DSN = dsn
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}

func simpleLogs(count int) pdata.Logs {
	logs := pdata.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	for i := 0; i < count; i++ {
		r := ill.LogRecords().AppendEmpty()
		r.SetTimestamp(pdata.NewTimestampFromTime(time.Now()))
		r.Attributes().InsertString("k", "v")
	}
	return logs
}

func mustPushLogsData(t *testing.T, exporter *clickhouseExporter, ld pdata.Logs) {
	err := exporter.pushLogsData(context.TODO(), ld)
	require.NoError(t, err)
}

const testDriverName = "clickhouse-test"

func initClickhouseTestServer(_ *testing.T, recorder recorder) {
	driverName = testDriverName
	sql.Register(testDriverName, &testClickhouseDriver{
		recorder: recorder,
	})
}

type recorder func(query string, values []driver.Value) error

type testClickhouseDriver struct {
	recorder recorder
}

func (t *testClickhouseDriver) Open(name string) (driver.Conn, error) {
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

func (*testClickhouseDriverConn) CheckNamedValue(v *driver.NamedValue) error {
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

func (t *testClickhouseDriverStmt) Query(args []driver.Value) (driver.Rows, error) {
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
