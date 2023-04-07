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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
)

type logsReceiver struct {
	config           *Config
	settings         receiver.CreateSettings
	createConnection dbProviderFunc
	createClient     clientProviderFunc
	queryReceivers   []logsQueryReceiver
}

func newLogsReceiver(
	config *Config,
	settings receiver.CreateSettings,
	sqlOpenerFunc sqlOpenerFunc,
	createClient clientProviderFunc,
) (*logsReceiver, error) {
	receiver := &logsReceiver{
		config:   config,
		settings: settings,
		createConnection: func() (*sql.DB, error) {
			return sqlOpenerFunc(config.Driver, config.DataSource)
		},
		createClient: createClient,
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
			id:    id,
			query: query,
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
	id    component.ID
	query Query
}

func (queryReceiver *logsQueryReceiver) start() {

}

func (queryReceiver *logsQueryReceiver) scrape(ctx context.Context) (plog.Logs, error) {
	return plog.NewLogs(), nil
}

func (queryReceiver *logsQueryReceiver) shutdown(ctx context.Context) error {
	return nil
}
