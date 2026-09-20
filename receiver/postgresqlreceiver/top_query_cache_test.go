// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"database/sql/driver"
	"fmt"
	"slices"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func TestTopQueryCacheRetainsFetchedStatements(t *testing.T) {
	columns := []string{
		callsColumnName, "datname", sharedBlksDirtiedColumnName, sharedBlksHitColumnName,
		sharedBlksReadColumnName, sharedBlksWrittenColumnName, tempBlksReadColumnName,
		tempBlksWrittenColumnName, "query", queryidColumnName, "rolname", rowsColumnName,
		totalExecTimeColumnName, totalPlanTimeColumnName,
	}
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 500
	cfg.TopQueryCollection.TopNQuery = 50
	cfg.TopQueryCollection.MaxExplainEachInterval = 0

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
	require.NoError(t, err)

	ids := make([]int, 500)
	for i := range ids {
		ids[i] = i
	}
	reversed := slices.Clone(ids)
	slices.Reverse(reversed)
	changed := slices.Clone(ids)
	for i := range changed[:250] {
		changed[i] = 500 + i
	}

	for _, tt := range []struct {
		name          string
		ids           []int
		increment     bool
		expectedCount int
	}{
		{name: "first scrape", ids: ids, expectedCount: 50},
		{name: "unchanged counters", ids: ids},
		{name: "reordered statements", ids: reversed},
		{name: "changed statement membership", ids: changed, expectedCount: 50},
		{name: "unchanged after membership change", ids: changed},
		{name: "one active statement", ids: changed, increment: true, expectedCount: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := sqlmock.NewRows(columns)
			for _, id := range tt.ids {
				values := map[string]driver.Value{
					"datname":         "postgres",
					"query":           "SELECT 1",
					"rolname":         "otel",
					queryidColumnName: fmt.Sprint(id),
				}
				for _, column := range columns {
					if _, exists := values[column]; !exists {
						values[column] = "10000"
					}
				}
				if tt.increment && id == 499 {
					values[callsColumnName] = "10003"
					values[rowsColumnName] = "10007"
					values[sharedBlksHitColumnName] = "10009"
					values[totalExecTimeColumnName] = "12500"
					values[totalPlanTimeColumnName] = "10500"
				}
				row := make([]driver.Value, len(columns))
				for i, column := range columns {
					row[i] = values[column]
				}
				rows.AddRow(row...)
			}
			mock.ExpectQuery("LIMIT 500").WillReturnRows(rows)
			logs, scrapeErr := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
			require.NoError(t, scrapeErr)
			require.NoError(t, mock.ExpectationsWereMet())
			require.Equal(t, tt.expectedCount, logs.LogRecordCount())
			if tt.increment {
				attrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()
				assert.Equal(t, "499", attrs[dbAttributePrefix+queryidColumnName])
				assert.Equal(t, int64(3), attrs[dbAttributePrefix+callsColumnName])
				assert.Equal(t, int64(7), attrs[dbAttributePrefix+rowsColumnName])
				assert.Equal(t, int64(9), attrs[dbAttributePrefix+sharedBlksHitColumnName])
				assert.Equal(t, 2.5, attrs[dbAttributePrefix+totalExecTimeColumnName])
				assert.Equal(t, 0.5, attrs[dbAttributePrefix+totalPlanTimeColumnName])
				assert.Equal(t, int64(0), attrs[dbAttributePrefix+sharedBlksReadColumnName])
			}
		})
	}
	mock.ExpectClose()
}
