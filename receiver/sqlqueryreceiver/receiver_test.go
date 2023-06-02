// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

func TestCreateReceiver(t *testing.T) {
	createReceiver := createReceiverFunc(fakeDBConnect, mkFakeClient)
	ctx := context.Background()
	receiver, err := createReceiver(
		ctx,
		receivertest.NewNopCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: 10 * time.Second,
			},
			Driver:     "mydriver",
			DataSource: "my-datasource",
			Queries: []Query{{
				SQL: "select * from foo",
				Metrics: []MetricCfg{{
					MetricName:  "my-metric",
					ValueColumn: "my-column",
				}},
			}},
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
}

func fakeDBConnect(string, string) (*sql.DB, error) {
	return nil, nil
}

func mkFakeClient(db, string, *zap.Logger) dbClient {
	return &fakeDBClient{stringMaps: [][]stringMap{{{"foo": "111"}}}}
}
