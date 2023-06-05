// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestDefaultClientCreation(t *testing.T) {
	_, err := newDefaultClient(componenttest.NewNopTelemetrySettings(), Config{
		Username:  "testuser",
		Password:  "testPassword",
		Account:   "testAccount",
		Schema:    "testSchema",
		Warehouse: "testWarehouse",
		Database:  "testDatabase",
		Role:      "testRole",
	})
	assert.Equal(t, nil, err)
}

// test query wrapper
func TestClientReadDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal("an error was not expected when opening mock db", err)
	}
	defer db.Close()

	q := "SELECT * FROM mocktable"
	rows := mock.NewRows([]string{"row1", "row2"}).AddRow(1, 3)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM mocktable`)).WillReturnRows(rows)

	client := snowflakeClient{
		client: db,
		logger: receivertest.NewNopCreateSettings().Logger,
	}

	ctx := context.Background()

	_, err = client.readDB(ctx, q)
	if err != nil {
		t.Errorf("Error during readDB: %s", err)
	}

	if err = mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestMetricQueries(t *testing.T) {
	tests := []struct {
		desc    string
		query   string
		columns []string
		params  []driver.Value
		expect  interface{}
	}{
		{
			desc:    "FetchBillingMetrics",
			query:   billingMetricsQuery,
			columns: []string{"service_type", "service_name", "virtualwarehouse_credit", "cloud_service", "totalcredit"},
			params:  []driver.Value{"t", "n", 1.0, 2.0, 3.0},
			expect: billingMetric{
				serviceType: sql.NullString{
					String: "t",
					Valid:  true,
				},
				serviceName: sql.NullString{
					String: "n",
					Valid:  true,
				},
				totalCloudService:            2.0,
				totalCredits:                 3.0,
				totalVirtualWarehouseCredits: 1.0,
			},
		},
		{
			desc:    "FetchWarehouseBillingMetrics",
			query:   warehouseBillingMetricsQuery,
			columns: []string{"wh_name", "virtual_wh", "cloud_service", "credit"},
			params:  []driver.Value{"n", 1.0, 2.0, 3.0},
			expect: whBillingMetric{
				warehouseName: sql.NullString{
					String: "n",
					Valid:  true,
				},
				totalCloudService:     2.0,
				totalCredit:           3.0,
				totalVirtualWarehouse: 1.0,
			},
		},
		{
			desc:    "FetchLoginMetrics",
			query:   loginMetricsQuery,
			columns: []string{"username", "error_message", "client_type", "is_success", "login_total"},
			params:  []driver.Value{"t", "n", "m", "l", 1},
			expect: loginMetric{
				userName: sql.NullString{
					String: "t",
					Valid:  true,
				},
				errorMessage: sql.NullString{
					String: "n",
					Valid:  true,
				},
				reportedClientType: sql.NullString{
					String: "m",
					Valid:  true,
				},
				isSuccess: sql.NullString{
					String: "l",
					Valid:  true,
				},
				loginsTotal: 1,
			},
		},
		{
			desc:    "FetchHighLevelQueryMetrics",
			query:   highLevelQueryMetricsQuery,
			columns: []string{"wh_name", "query_executed", "queue_overload", "queue_provision", "query_blocked"},
			params:  []driver.Value{"t", 0.0, 1.0, 2.0, 3.0},
			expect: hlQueryMetric{
				warehouseName: sql.NullString{
					String: "t",
					Valid:  true,
				},
				avgQueryExecuted:        0.0,
				avgQueryBlocked:         3.0,
				avgQueryQueuedOverload:  1.0,
				avgQueryQueuedProvision: 2.0,
			},
		},
		{
			desc:  "FetchDbMetrics",
			query: dbMetricsQuery,
			columns: []string{"schemaname", "execution_status", "error_message",
				"query_type", "wh_name", "db_name", "wh_size", "username",
				"count_queryid", "queued_overload", "queued_repair", "queued_provision",
				"total_elapsed", "execution_time", "comp_time", "bytes_scanned",
				"bytes_written", "bytes_deleted", "bytes_spilled_local", "bytes_spilled_remote",
				"percentage_cache", "partitions_scanned", "rows_unloaded", "rows_deleted",
				"rows_updated", "rows_inserted", "rows_produced"},
			params: []driver.Value{"a", "b", "c", "d", "e", "f", "g", "h", 1, 2.0, 3.0, 4.0, 5.0, 6.0,
				7.0, 8.0, 9.0, 10.0, 11.0, 12.0, 13.0, 14.0, 15.0, 16.0, 17.0, 18.0, 19.0},
			expect: dbMetric{
				attributes: dbMetricAttributes{
					userName: sql.NullString{
						String: "h",
						Valid:  true,
					},
					schemaName: sql.NullString{
						String: "a",
						Valid:  true,
					},
					executionStatus: sql.NullString{
						String: "b",
						Valid:  true,
					},
					errorMessage: sql.NullString{
						String: "c",
						Valid:  true,
					},
					queryType: sql.NullString{
						String: "d",
						Valid:  true,
					},
					warehouseName: sql.NullString{
						String: "e",
						Valid:  true,
					},
					warehouseSize: sql.NullString{
						String: "g",
						Valid:  true,
					},
					databaseName: sql.NullString{
						String: "f",
						Valid:  true,
					},
				},
				databaseQueryCount:        1,
				avgBytesScanned:           8.0,
				avgBytesDeleted:           10.0,
				avgBytesSpilledRemote:     12.0,
				avgBytesSpilledLocal:      11.0,
				avgBytesWritten:           9.0,
				avgCompilationTime:        7.0,
				avgDataScannedCache:       13.0,
				avgExecutionTime:          6.0,
				avgPartitionsScanned:      14.0,
				avgQueuedOverloadTime:     2.0,
				avgQueuedProvisioningTime: 4.0,
				avgQueuedRepairTime:       3.0,
				avgRowsInserted:           18.0,
				avgRowsDeleted:            16.0,
				avgRowsProduced:           19.0,
				avgRowsUnloaded:           15.0,
				avgRowsUpdated:            17.0,
				avgTotalElapsedTime:       5.0,
			},
		},
		{
			desc:    "FetchSessionMetrics",
			query:   sessionMetricsQuery,
			columns: []string{"username", "disctinct_id"},
			params:  []driver.Value{"t", 3.0},
			expect: sessionMetric{
				userName: sql.NullString{
					String: "t",
					Valid:  true,
				},
				distinctSessionID: 3.0,
			},
		},
		{
			desc:    "FetchSnowpipeMetrics",
			query:   snowpipeMetricsQuery,
			columns: []string{"pipe_name", "credits_used", "bytes_inserted", "files_inserted"},
			params:  []driver.Value{"t", 1.0, 2.0, 3.0},
			expect: snowpipeMetric{
				pipeName: sql.NullString{
					String: "t",
					Valid:  true,
				},
				creditsUsed:   1.0,
				bytesInserted: 2.0,
				filesInserted: 3.0,
			},
		},
		{
			desc:    "FetchStorageMetrics",
			query:   storageMetricsQuery,
			columns: []string{"storage_bytes", "stage_bytes", "failsafe_bytes"},
			params:  []driver.Value{1, 2, 3},
			expect: storageMetric{
				storageBytes:  1,
				stageBytes:    2,
				failsafeBytes: 3,
			},
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.desc, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal("an error was not expected when opening mock db", err)
			}

			rows := mock.NewRows(test.columns).AddRow(test.params...)
			mock.ExpectQuery(test.query).WillReturnRows(rows)
			defer db.Close()

			client := snowflakeClient{
				client: db,
				logger: receivertest.NewNopCreateSettings().Logger,
			}
			ctx := context.Background()

			// iteratively call each client method with the correct db mock
			clientVal := reflect.ValueOf(&client)
			clientObj := reflect.Indirect(clientVal)
			returnVal := clientObj.MethodByName(test.desc).Call([]reflect.Value{reflect.ValueOf(ctx)})

			// GetMetric functions return a slice of type <metricType> but since we only have one
			// row we can safely just compare the first elem from the reflected slice
			metric := returnVal[0].Type().Elem().Elem()
			if err, ok := returnVal[1].Interface().(error); ok && err != nil {
				t.Errorf("DB error %v", err)
			}

			assert.Equal(t, reflect.TypeOf(test.expect), metric)
		})
	}
}
