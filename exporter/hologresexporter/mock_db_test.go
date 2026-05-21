// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// execMatcher is invoked per Exec call; if it returns a non-nil err, that error
// is returned. The matcher receives the SQL string. The first matcher whose
// substring is contained in the SQL is consumed.
type execExpectation struct {
	sqlContains string
	err         error
}

// copyFromCall records arguments observed by a CopyFrom invocation.
type copyFromCall struct {
	table   pgx.Identifier
	columns []string
	rows    [][]any
}

// mockPgxDB is a lightweight in-memory implementation of the pgxDB interface
// used for unit tests. It records Exec/CopyFrom invocations and lets the test
// configure errors to return.
type mockPgxDB struct {
	mu sync.Mutex

	// Default error returned by Exec when no matching expectation is set.
	defaultExecErr error

	// Sequence of expectations consumed in order. When sqlContains matches,
	// the expectation is consumed and the err returned.
	execExpectations []execExpectation

	// Recorded calls.
	execCalls     []string
	copyFromCalls []copyFromCall

	// Errors to return for CopyFrom calls (FIFO). When empty, returns nil.
	copyFromErrs []error

	closed   bool
	pingErr  error
	pingDone bool
}

func newMockPgxDB() *mockPgxDB {
	return &mockPgxDB{}
}

// expectExec queues an expectation that an Exec call's SQL contains the given
// substring; the provided error is returned when matched.
func (m *mockPgxDB) expectExec(sqlContains string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execExpectations = append(m.execExpectations, execExpectation{sqlContains: sqlContains, err: err})
}

// queueCopyFromErr queues an error to be returned by the next CopyFrom call.
func (m *mockPgxDB) queueCopyFromErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.copyFromErrs = append(m.copyFromErrs, err)
}

func (m *mockPgxDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls = append(m.execCalls, sql)

	// Find a matching expectation and consume it.
	for i, exp := range m.execExpectations {
		if strings.Contains(sql, exp.sqlContains) {
			m.execExpectations = append(m.execExpectations[:i], m.execExpectations[i+1:]...)
			return pgconn.CommandTag{}, exp.err
		}
	}

	return pgconn.CommandTag{}, m.defaultExecErr
}

func (m *mockPgxDB) CopyFrom(_ context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var rows [][]any
	for rowSrc.Next() {
		values, err := rowSrc.Values()
		if err != nil {
			return 0, err
		}
		// Copy slice to detach from source state.
		row := make([]any, len(values))
		copy(row, values)
		rows = append(rows, row)
	}
	if err := rowSrc.Err(); err != nil {
		return 0, err
	}

	m.copyFromCalls = append(m.copyFromCalls, copyFromCall{
		table:   tableName,
		columns: columnNames,
		rows:    rows,
	})

	if len(m.copyFromErrs) > 0 {
		err := m.copyFromErrs[0]
		m.copyFromErrs = m.copyFromErrs[1:]
		if err != nil {
			return 0, err
		}
	}
	return int64(len(rows)), nil
}

func (m *mockPgxDB) Ping(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pingDone = true
	return m.pingErr
}

func (m *mockPgxDB) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}
