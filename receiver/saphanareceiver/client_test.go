// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package saphanareceiver

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testResultWrapper struct {
	contents [][]string
	current  int
}

func (m *testResultWrapper) Next() bool {
	return m.current < len(m.contents)
}

func (m *testResultWrapper) Scan(dest ...interface{}) error {
	for i, output := range dest {
		d, _ := output.(*string)
		*d = m.contents[m.current][i]
	}

	m.current = m.current + 1
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

func (w *testDBWrapper) mockQueryResult(query string, results [][]string, err error) {
	resultWrapper := &testResultWrapper{
		contents: results,
		current:  0,
	}

	w.On("QueryContext", query).Return(resultWrapper, err)
}

type testConnectionFactory struct {
	dbWrapper testDBWrapper
}

func (m *testConnectionFactory) getConnection(c driver.Connector) dbWrapper {
	return &m.dbWrapper
}

func TestBasicConnectAndClose(t *testing.T) {
	dbWrapper := testDBWrapper{}
	dbWrapper.On("PingContext").Return(nil)
	dbWrapper.On("Close").Return(nil)

	factory := &testConnectionFactory{dbWrapper}
	client := newSapHanaClient(createDefaultConfig().(*Config), factory)

	require.NoError(t, client.Connect(context.TODO()))
	require.NoError(t, client.Close())
}

func TestFailedPing(t *testing.T) {
	dbWrapper := testDBWrapper{}
	dbWrapper.On("PingContext").Return(errors.New("Coult not ping host"))
	// Should pass without this mock, as db should not be retained if it could not connect
	// so the client.Close() should be effectively a no-op
	// dbWrapper.On("Close").Return(nil)

	factory := &testConnectionFactory{dbWrapper}
	client := newSapHanaClient(createDefaultConfig().(*Config), factory)

	require.Error(t, client.Connect(context.TODO()))
	require.NoError(t, client.Close())
}

func TestSimpleQueryOutput(t *testing.T) {
	dbWrapper := testDBWrapper{}
	dbWrapper.On("PingContext").Return(nil)
	dbWrapper.On("Close").Return(nil)

	dbWrapper.mockQueryResult("SELECT 1=1", [][]string{
		{"my_id", "dead", "1", "8.044"},
		{"your_id", "alive", "2", "600.1"},
	}, nil)

	client := newSapHanaClient(createDefaultConfig().(*Config), &testConnectionFactory{dbWrapper})
	require.NoError(t, client.Connect(context.TODO()))

	query := &monitoringQuery{
		query:         "SELECT 1=1",
		orderedLabels: []string{"id", "state"},
		orderedStats: []queryStat{
			{
				key: "value",
				addIntMetricFunction: func(shs *sapHanaScraper, t pdata.Timestamp, i int64, m map[string]string) {
					// Function is a no-op as it's just used to know how to parse the value (into int or double)
				},
			},
			{
				key: "rate",
				addDoubleMetricFunction: func(shs *sapHanaScraper, t pdata.Timestamp, f float64, m map[string]string) {
					// Function is a no-op as it's just used to know how to parse the value (into int or double)
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
