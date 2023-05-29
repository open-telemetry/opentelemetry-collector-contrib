// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package saphanareceiver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

type testResultWrapper struct {
	contents [][]sql.NullString
	current  int
}

func (m *testResultWrapper) Next() bool {
	return m.current < len(m.contents)
}

func (m *testResultWrapper) Scan(dest ...interface{}) error {
	for i, output := range dest {
		d, _ := output.(*sql.NullString)
		*d = m.contents[m.current][i]
	}

	m.current++
	return nil
}

func (m *testResultWrapper) Close() error {
	return nil
}

type testDBWrapper struct{ mock.Mock }

func (m *testDBWrapper) PingContext(ctx context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *testDBWrapper) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *testDBWrapper) QueryContext(ctx context.Context, query string) (resultWrapper, error) {
	args := m.Called(query)
	result := args.Get(0).(*testResultWrapper)
	err := args.Error(1)
	return result, err
}

func (m *testDBWrapper) mockQueryResult(query string, results [][]*string, err error) {
	var nullableResult [][]sql.NullString
	for _, row := range results {
		var nullableRow []sql.NullString
		for _, field := range row {
			if field == nil {
				nullableRow = append(nullableRow, sql.NullString{Valid: false})
			} else {
				nullableRow = append(nullableRow, sql.NullString{Valid: true, String: *field})
			}
		}

		nullableResult = append(nullableResult, nullableRow)
	}
	resultWrapper := &testResultWrapper{
		contents: nullableResult,
		current:  0,
	}

	m.On("QueryContext", query).Return(resultWrapper, err)
}

type testConnectionFactory struct {
	dbWrapper *testDBWrapper
}

func (m *testConnectionFactory) getConnection(c driver.Connector) dbWrapper {
	return m.dbWrapper
}

func str(str string) *string {
	return &str
}

func TestBasicConnectAndClose(t *testing.T) {
	dbWrapper := &testDBWrapper{}
	dbWrapper.On("PingContext").Return(nil)
	dbWrapper.On("Close").Return(nil)

	factory := &testConnectionFactory{dbWrapper}
	client := newSapHanaClient(createDefaultConfig().(*Config), factory)

	require.NoError(t, client.Connect(context.TODO()))
	require.NoError(t, client.Close())
}

func TestFailedPing(t *testing.T) {
	dbWrapper := &testDBWrapper{}
	dbWrapper.On("PingContext").Return(errors.New("Coult not ping host"))
	dbWrapper.On("Close").Return(nil)

	factory := &testConnectionFactory{dbWrapper}
	client := newSapHanaClient(createDefaultConfig().(*Config), factory)

	require.Error(t, client.Connect(context.TODO()))
	require.NoError(t, client.Close())
}

func TestSimpleQueryOutput(t *testing.T) {
	dbWrapper := &testDBWrapper{}
	dbWrapper.On("PingContext").Return(nil)
	dbWrapper.On("Close").Return(nil)

	dbWrapper.mockQueryResult("SELECT 1=1", [][]*string{
		{str("my_id"), str("dead"), str("1"), str("8.044")},
		{str("your_id"), str("alive"), str("2"), str("600.1")},
	}, nil)

	client := newSapHanaClient(createDefaultConfig().(*Config), &testConnectionFactory{dbWrapper})
	require.NoError(t, client.Connect(context.TODO()))

	query := &monitoringQuery{
		query:               "SELECT 1=1",
		orderedMetricLabels: []string{"id", "state"},
		orderedStats: []queryStat{
			{
				key: "value",
				addMetricFunction: func(mb *metadata.MetricsBuilder, t pcommon.Timestamp, val string,
					m map[string]string) error {
					// Function is a no-op as it's not required for this test
					return nil
				},
			},
			{
				key: "rate",
				addMetricFunction: func(mb *metadata.MetricsBuilder, t pcommon.Timestamp, val string,
					m map[string]string) error {
					// Function is a no-op as it's not required for this test
					return nil
				},
			},
		},
	}

	results, err := client.collectDataFromQuery(context.TODO(), query)
	require.NoError(t, err)
	require.Equal(t, []map[string]string{
		{
			"id":    "my_id",
			"state": "dead",
			"value": "1",
			"rate":  "8.044",
		},
		{
			"id":    "your_id",
			"state": "alive",
			"value": "2",
			"rate":  "600.1",
		},
	}, results)

	require.NoError(t, client.Close())
}

func TestNullOutput(t *testing.T) {
	dbWrapper := &testDBWrapper{}
	dbWrapper.On("PingContext").Return(nil)
	dbWrapper.On("Close").Return(nil)

	dbWrapper.mockQueryResult("SELECT 1=1", [][]*string{
		{str("my_id"), str("dead"), str("1"), nil},
		{nil, str("live"), str("3"), str("123.123")},
	}, nil)

	client := newSapHanaClient(createDefaultConfig().(*Config), &testConnectionFactory{dbWrapper})
	require.NoError(t, client.Connect(context.TODO()))

	query := &monitoringQuery{
		query:               "SELECT 1=1",
		orderedMetricLabels: []string{"id", "state"},
		orderedStats: []queryStat{
			{
				key: "value",
				addMetricFunction: func(mb *metadata.MetricsBuilder, t pcommon.Timestamp, val string,
					m map[string]string) error {
					// Function is a no-op as it's not required for this test
					return nil
				},
			},
			{
				key: "rate",
				addMetricFunction: func(mb *metadata.MetricsBuilder, t pcommon.Timestamp, val string,
					m map[string]string) error {
					// Function is a no-op as it's not required for this test
					return nil
				},
			},
		},
	}

	results, err := client.collectDataFromQuery(context.TODO(), query)
	// Error expected for second row, but data is also returned
	require.Contains(t, err.Error(), "database row NULL value for required metric label id")
	require.Equal(t, []map[string]string{
		{
			"id":    "my_id",
			"state": "dead",
			"value": "1",
		},
	}, results)

	require.NoError(t, client.Close())
}
