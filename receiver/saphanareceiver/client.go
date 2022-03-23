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

package saphanareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	sapdriver "github.com/SAP/go-hdb/driver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

// Interface for a SAP HANA client. Implementation can be faked for testing.
type client interface {
	Connect(ctx context.Context) error
	collectDataFromQuery(ctx context.Context, query *monitoringQuery) ([]map[string]string, error)
	Close() error
}

// Wraps the result of a query so that it can be mocked in tests
type resultWrapper interface {
	Scan(dest ...interface{}) error
	Close() error
	Next() bool
}

// Wraps the sqlDB interface so that it can be mocked in tests
type dbWrapper interface {
	PingContext(ctx context.Context) error
	Close() error
	QueryContext(ctx context.Context, query string) (resultWrapper, error)
}

type standardResultWrapper struct {
	rows *sql.Rows
}

func (w *standardResultWrapper) Next() bool {
	return w.rows.Next()
}

func (w *standardResultWrapper) Scan(dest ...interface{}) error {
	return w.rows.Scan(dest...)
}

func (w *standardResultWrapper) Close() error {
	return w.rows.Close()
}

type standardDBWrapper struct {
	db *sql.DB
}

func (w *standardDBWrapper) Close() error {
	return w.db.Close()
}

func (w *standardDBWrapper) PingContext(ctx context.Context) error {
	return w.db.PingContext(ctx)
}

func (w *standardDBWrapper) QueryContext(ctx context.Context, query string) (resultWrapper, error) {
	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	resultWrapper := standardResultWrapper{rows}
	return &resultWrapper, nil
}

// Wraps the creation of a sqlDB so that it can be mocked in tests
type sapHanaConnectionFactory interface {
	getConnection(c driver.Connector) dbWrapper
}

type defaultConnectionFactory struct{}

func (f *defaultConnectionFactory) getConnection(c driver.Connector) dbWrapper {
	wrapper := standardDBWrapper{db: sql.OpenDB((c))}
	return &wrapper
}

// Wraps a SAP HANA database connection, implements `client` interface.
type sapHanaClient struct {
	receiverConfig    *Config
	connectionFactory sapHanaConnectionFactory
	client            dbWrapper
}

var _ client = (*sapHanaClient)(nil)

// Creates a SAP HANA database client
func newSapHanaClient(cfg *Config, factory sapHanaConnectionFactory) client {
	return &sapHanaClient{
		receiverConfig:    cfg,
		connectionFactory: factory,
	}
}

func (c *sapHanaClient) Connect(ctx context.Context) error {
	connector, err := sapdriver.NewDSNConnector(fmt.Sprintf("hdb://%s:%s@%s", c.receiverConfig.Username, c.receiverConfig.Password, c.receiverConfig.TCPAddr.Endpoint))
	if err != nil {
		return fmt.Errorf("error generating DSN for SAP HANA connection: %w", err)
	}

	if tls, err := c.receiverConfig.TLSClientSetting.LoadTLSConfig(); err != nil {
		return fmt.Errorf("error generating TLS config for SAP HANA connection: %w", err)
	} else {
		connector.SetTLSConfig(tls)
	}

	connector.SetApplicationName("OpenTelemetry Collector")
	client := c.connectionFactory.getConnection(connector)

	err = client.PingContext(ctx)
	if err == nil {
		c.client = client
	}

	return err
}

func (c *sapHanaClient) Close() error {
	if c.client != nil {
		client := c.client
		c.client = nil
		return client.Close()
	}
	return nil
}

func (c *sapHanaClient) collectDataFromQuery(ctx context.Context, query *monitoringQuery) ([]map[string]string, error) {
	rows, err := c.client.QueryContext(ctx, query.query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	errors := scrapererror.ScrapeErrors{}
	data := []map[string]string{}
	for rows.Next() {
		rowFields := make([]interface{}, 0)

		// Build a list of addresses that rows.Scan will load column data into
		for range query.columns() {
			var val string
			rowFields = append(rowFields, &val)
		}

		if err := rows.Scan(rowFields...); err != nil {
			return nil, err
		}

		values := map[string]string{}
		for _, label := range query.columns() {
			v, err := convertInterfaceToString(rowFields[0])
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			values[label] = v
			rowFields = rowFields[1:]
		}

		data = append(data, values)
	}
	return data, errors.Combine()
}

func convertInterfaceToString(input interface{}) (string, error) {
	if val, ok := input.(*string); ok {
		return *val, nil
	}
	return "", errors.New("issue converting interface into string")
}
