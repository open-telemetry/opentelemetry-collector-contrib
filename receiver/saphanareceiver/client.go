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
	"errors"
	"fmt"

	"github.com/SAP/go-hdb/driver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

// Interface for a SAP HANA client. Implementation can be faked for testing.
type client interface {
	Connect() error
	collectDataFromQuery(ctx context.Context, query *monitoringQuery) ([]map[string]string, error)
	Close() error
}

// Wraps a SAP HANA database connection, implements `client` interface.
type sapHanaClient struct {
	receiverConfig *Config
	client         *sql.DB
}

var _ client = (*sapHanaClient)(nil)

// Creates a SAP HANA database client
func newSapHanaClient(cfg *Config) client {
	return &sapHanaClient{
		receiverConfig: cfg,
	}
}

func (c *sapHanaClient) Connect() error {
	connector, err := driver.NewDSNConnector(fmt.Sprintf("hdb://%s:%s@%s", c.receiverConfig.Username, c.receiverConfig.Password, c.receiverConfig.TCPAddr.Endpoint))
	if err != nil {
		return fmt.Errorf("error generating DSN for SAP HANA connection: %w", err)
	}

	if tls, err := c.receiverConfig.TLSClientSetting.LoadTLSConfig(); err != nil {
		return fmt.Errorf("error generating TLS config for SAP HANA connection: %w", err)
	} else {
		connector.SetTLSConfig(tls)
	}

	connector.SetApplicationName("OpenTelemetry Collector")
	clientDB := sql.OpenDB(connector)
	if err != nil {
		return fmt.Errorf("unable to connect to SAP HANA database: %w", err)
	}
	c.client = clientDB
	return nil
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
