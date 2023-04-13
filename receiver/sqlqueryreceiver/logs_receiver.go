// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
)

type logsReceiver struct {
	config           *Config
	settings         receiver.CreateSettings
	createConnection dbProviderFunc
	createClient     clientProviderFunc
	queryReceivers   []logsQueryReceiver
	nextConsumer     consumer.Logs
}

func newLogsReceiver(
	config *Config,
	settings receiver.CreateSettings,
	sqlOpenerFunc sqlOpenerFunc,
	createClient clientProviderFunc,
	nextConsumer consumer.Logs,
) (*logsReceiver, error) {
	receiver := &logsReceiver{
		config:   config,
		settings: settings,
		createConnection: func() (*sql.DB, error) {
			return sqlOpenerFunc(config.Driver, config.DataSource)
		},
		createClient: createClient,
		nextConsumer: nextConsumer,
	}

	receiver.createQueryReceivers()

	return receiver, nil
}

func (receiver *logsReceiver) createQueryReceivers() {
	for i, query := range receiver.config.Queries {
		if len(query.Logs) == 0 {
			continue
		}
		id := component.NewIDWithName("sqlqueryreceiver", fmt.Sprintf("query-%d: %s", i, query.SQL))
		queryReceiver := logsQueryReceiver{
			id:                 id,
			query:              query,
			nextConsumer:       receiver.nextConsumer,
			clientProviderFunc: receiver.createClient,
			dbProviderFunc:     receiver.createConnection,
		}
		receiver.queryReceivers = append(receiver.queryReceivers, queryReceiver)
	}
}

func (receiver *logsReceiver) Start(ctx context.Context, host component.Host) error {
	for _, queryReceiver := range receiver.queryReceivers {
		queryReceiver.start()
	}
	return nil
}

func (receiver *logsReceiver) Shutdown(ctx context.Context) error {
	return nil
}

type logsQueryReceiver struct {
	id                 component.ID
	query              Query
	nextConsumer       consumer.Logs
	clientProviderFunc clientProviderFunc
	dbProviderFunc     dbProviderFunc
	client             dbClient
}

func (queryReceiver *logsQueryReceiver) start() {
	db, err := queryReceiver.dbProviderFunc()
	if err != nil {
		//TODO: zalogować jakiś piękny błąd
		panic(err)
	}
	queryReceiver.client = queryReceiver.clientProviderFunc(dbWrapper{db}, queryReceiver.query.SQL, nil)
	queryReceiver.scrape(context.Background())
}

func (queryReceiver *logsQueryReceiver) scrape(ctx context.Context) (plog.Logs, error) {
	out := plog.NewLogs()
	rows, err := queryReceiver.client.queryRows(ctx)
	if err != nil {
		return out, fmt.Errorf("scraper: %w", err)
	}
	for i, row := range rows {
		fmt.Println(i, row)
	}
	aaa := out.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	kiloStr := strings.Repeat("x", 10*1024)
	aaa.SetSeverityText(kiloStr)
	queryReceiver.nextConsumer.ConsumeLogs(ctx, out)
	return plog.NewLogs(), nil
}

func (queryReceiver *logsQueryReceiver) shutdown(ctx context.Context) error {
	return nil
}
