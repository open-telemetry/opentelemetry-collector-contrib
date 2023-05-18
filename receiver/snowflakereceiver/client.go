// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"context"
	"database/sql"
	"fmt"

	sf "github.com/snowflakedb/gosnowflake"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// queries
var (
	billingMetricsQuery          = "select SERVICE_TYPE, NAME, sum(CREDITS_USED_COMPUTE), sum(CREDITS_USED_CLOUD_SERVICES), sum(CREDITS_USED) from METERING_HISTORY where start_time >= DATEADD(hour, -24, current_timestamp()) group by 1, 2;"
	warehouseBillingMetricsQuery = "select WAREHOUSE_NAME, sum(CREDITS_USED_COMPUTE), sum(CREDITS_USED_CLOUD_SERVICES), sum(CREDITS_USED) from WAREHOUSE_METERING_HISTORY where start_time >= DATEADD(hour, -24, current_timestamp()) group by 1;"
	loginMetricsQuery            = "select USER_NAME, ERROR_MESSAGE, REPORTED_CLIENT_TYPE, IS_SUCCESS, count(*) from LOGIN_HISTORY where event_timestamp >= DATEADD(hour, -24, current_timestamp()) group by 1, 2, 3, 4;"
	highLevelQueryMetricsQuery   = "select WAREHOUSE_NAME, AVG(AVG_RUNNING), AVG(AVG_QUEUED_LOAD), AVG(AVG_QUEUED_PROVISIONING), AVG(AVG_BLOCKED) from WAREHOUSE_LOAD_HISTORY where start_time >= DATEADD(hour, -24, current_timestamp()) group by 1;"
	dbMetricsQuery               = "select SCHEMA_NAME, EXECUTION_STATUS, ERROR_MESSAGE, QUERY_TYPE, WAREHOUSE_NAME, DATABASE_NAME, WAREHOUSE_SIZE, USER_NAME, COUNT(QUERY_ID), AVG(queued_overload_time), AVG(queued_repair_time), AVG(queued_provisioning_time), AVG(TOTAL_ELAPSED_TIME), AVG(EXECUTION_TIME), AVG(COMPILATION_TIME), AVG(BYTES_SCANNED), AVG(BYTES_WRITTEN), AVG(BYTES_DELETED), AVG(BYTES_SPILLED_TO_LOCAL_STORAGE), AVG(BYTES_SPILLED_TO_REMOTE_STORAGE), AVG(PERCENTAGE_SCANNED_FROM_CACHE), AVG(PARTITIONS_SCANNED), AVG(ROWS_UNLOADED), AVG(ROWS_DELETED), AVG(ROWS_UPDATED), AVG(ROWS_INSERTED), AVG(COALESCE(ROWS_PRODUCED,0)) from QUERY_HISTORY where start_time >= DATEADD(hour, -24, current_timestamp()) group by 1, 2, 3, 4, 5, 6, 7, 8;"
	sessionMetricsQuery          = "select USER_NAME, count(distinct(SESSION_ID)) from Sessions where created_on >= DATEADD(hour, -24, current_timestamp()) group by 1;"
	snowpipeMetricsQuery         = "select pipe_name, sum(credits_used), sum(bytes_inserted), sum(files_inserted) from pipe_usage_history where start_time >= DATEADD(hour, -24, current_timestamp()) group by 1;"
	storageMetricsQuery          = "select STORAGE_BYTES, STAGE_BYTES, FAILSAFE_BYTES from STORAGE_USAGE ORDER BY USAGE_DATE DESC LIMIT 1;"
)

// snowflake client is comprised of a sql.DB (the proper 'client' in question),
// a connection string (dsn), a collection of queries (built from which metrics are enabled),
// and a logger
type snowflakeClient struct {
	client *sql.DB
	dsn    *string
	logger *zap.Logger
}

// build snowflake db connection string
func buildDSN(cfg Config) string {
	conf := &sf.Config{
		Account:   cfg.Account,
		User:      cfg.Username,
		Password:  cfg.Password,
		Database:  cfg.Database,
		Schema:    cfg.Schema,
		Role:      cfg.Role,
		Warehouse: cfg.Warehouse,
	}

	dsn, err := sf.DSN(conf)
	if err != nil {
		print("%v", err)
	}

	return dsn
}

func newDefaultClient(settings component.TelemetrySettings, c Config) (*snowflakeClient, error) {
	dsn := buildDSN(c)
	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, err
	}

	return &snowflakeClient{
		client: db,
		dsn:    &dsn,
		logger: settings.Logger,
	}, nil
}

// queries database and returns resulting rows
func (c snowflakeClient) readDB(ctx context.Context, q string) (*sql.Rows, error) {
	rows, err := c.client.QueryContext(ctx, q)
	if err != nil {
		error := fmt.Sprintf("Query failed with %v", err)
		c.logger.Error(error)
		return nil, err
	}

	return rows, nil
}

// these wrap readDB and return the associated data type which the scraper will
// use to generate and emit metrics. Which of these are called will be based on which metrics
// are enabled in the Config (default is all of them)
func (c snowflakeClient) FetchBillingMetrics(ctx context.Context) (*[]billingMetric, error) {
	rows, err := c.readDB(ctx, billingMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", billingMetricsQuery)
		return nil, err
	}

	var res []billingMetric

	for rows.Next() {
		var serviceType, serviceName sql.NullString
		var totalCloudService, totalTotalCredits, totalVirtualWarehouseCredits float64

		err := rows.Scan(&serviceType, &serviceName, &totalVirtualWarehouseCredits, &totalCloudService, &totalTotalCredits)
		if err != nil {
			return nil, err
		}

		res = append(res, billingMetric{
			serviceType:                  serviceType,
			serviceName:                  serviceName,
			totalCloudService:            totalCloudService,
			totalCredits:                 totalTotalCredits,
			totalVirtualWarehouseCredits: totalVirtualWarehouseCredits,
		})
	}
	return &res, nil
}

func (c snowflakeClient) FetchWarehouseBillingMetrics(ctx context.Context) (*[]whBillingMetric, error) {
	rows, err := c.readDB(ctx, warehouseBillingMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", warehouseBillingMetricsQuery)
		return nil, err
	}

	var res []whBillingMetric

	for rows.Next() {
		var warehouseName sql.NullString
		var totalCloudService, totalTotalCredit, totalVirtualWarehouse float64

		err := rows.Scan(&warehouseName, &totalVirtualWarehouse, &totalCloudService, &totalTotalCredit)
		if err != nil {
			return nil, err
		}
		res = append(res, whBillingMetric{
			warehouseName:         warehouseName,
			totalCloudService:     totalCloudService,
			totalCredit:           totalTotalCredit,
			totalVirtualWarehouse: totalVirtualWarehouse,
		})
	}
	return &res, nil
}

func (c snowflakeClient) FetchLoginMetrics(ctx context.Context) (*[]loginMetric, error) {
	rows, err := c.readDB(ctx, loginMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", loginMetricsQuery)
		return nil, err
	}

	var res []loginMetric

	for rows.Next() {
		var (
			userName           sql.NullString
			errorMessage       sql.NullString
			reportedClientType sql.NullString
			isSuccess          sql.NullString
		)
		var loginsTotal int64

		err := rows.Scan(&userName, &errorMessage, &reportedClientType, &isSuccess, &loginsTotal)
		if err != nil {
			return nil, err
		}

		res = append(res, loginMetric{
			userName:           userName,
			loginsTotal:        loginsTotal,
			errorMessage:       errorMessage,
			reportedClientType: reportedClientType,
			isSuccess:          isSuccess,
		})
	}
	return &res, nil
}

func (c snowflakeClient) FetchHighLevelQueryMetrics(ctx context.Context) (*[]hlQueryMetric, error) {
	rows, err := c.readDB(ctx, highLevelQueryMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", highLevelQueryMetricsQuery)
		return nil, err
	}

	var res []hlQueryMetric

	for rows.Next() {
		var (
			avgQueryExecuted        float64
			avgQueryBlocked         float64
			avgQueryQueuedOverload  float64
			avgQueryQueuedProvision float64
		)
		var warehouseName sql.NullString

		err := rows.Scan(&warehouseName, &avgQueryExecuted, &avgQueryQueuedOverload,
			&avgQueryQueuedProvision, &avgQueryBlocked)
		if err != nil {
			return nil, err
		}

		res = append(res,
			hlQueryMetric{
				warehouseName:           warehouseName,
				avgQueryExecuted:        avgQueryExecuted,
				avgQueryBlocked:         avgQueryBlocked,
				avgQueryQueuedOverload:  avgQueryQueuedOverload,
				avgQueryQueuedProvision: avgQueryQueuedProvision,
			})
	}

	return &res, nil
}

func (c snowflakeClient) FetchDbMetrics(ctx context.Context) (*[]dbMetric, error) {
	rows, err := c.readDB(ctx, dbMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", dbMetricsQuery)
		return nil, err
	}

	var res []dbMetric

	for rows.Next() {
		var (
			schemaName      sql.NullString
			executionStatus sql.NullString
			errorMessage    sql.NullString
			queryType       sql.NullString
			warehouseName   sql.NullString
			databaseName    sql.NullString
			warehouseSize   sql.NullString
			userName        sql.NullString
		)
		var (
			avgBytesScanned           float64
			avgDataScannedCache       float64
			avgQueuedOverloadTime     float64
			avgQueuedProvisioningTime float64
			avgQueuedRepairTime       float64
			databaseQueryCount        int64
			avgBytesDeleted           float64
			avgBytesSpilledRemote     float64
			avgBytesSpilledLocal      float64
			avgBytesWritten           float64
			avgCompilationTime        float64
			avgExecutionTime          float64
			avgPartitionsScanned      float64
			avgRowsInserted           float64
			avgRowsDeleted            float64
			avgRowsProduced           float64
			avgRowsUnloaded           float64
			avgRowsUpdated            float64
			avgTotalElapsedTime       float64
		)
		var attributes dbMetricAttributes
		var db dbMetric

		err := rows.Scan(&schemaName, &executionStatus,
			&errorMessage, &queryType, &warehouseName, &databaseName,
			&warehouseSize, &userName, &databaseQueryCount, &avgQueuedOverloadTime,
			&avgQueuedRepairTime, &avgQueuedProvisioningTime, &avgTotalElapsedTime,
			&avgExecutionTime, &avgCompilationTime, &avgBytesScanned, &avgBytesWritten,
			&avgBytesDeleted, &avgBytesSpilledLocal, &avgBytesSpilledRemote,
			&avgDataScannedCache, &avgPartitionsScanned, &avgRowsUnloaded,
			&avgRowsDeleted, &avgRowsUpdated, &avgRowsInserted, &avgRowsProduced)
		if err != nil {
			return nil, err
		}
		attributes = dbMetricAttributes{
			userName:        userName,
			schemaName:      schemaName,
			executionStatus: executionStatus,
			errorMessage:    errorMessage,
			queryType:       queryType,
			warehouseName:   warehouseName,
			databaseName:    databaseName,
			warehouseSize:   warehouseSize,
		}
		db = dbMetric{
			attributes:                attributes,
			databaseQueryCount:        databaseQueryCount,
			avgBytesScanned:           avgBytesScanned,
			avgBytesDeleted:           avgBytesDeleted,
			avgBytesSpilledRemote:     avgBytesSpilledRemote,
			avgBytesSpilledLocal:      avgBytesSpilledLocal,
			avgBytesWritten:           avgBytesWritten,
			avgCompilationTime:        avgCompilationTime,
			avgDataScannedCache:       avgDataScannedCache,
			avgExecutionTime:          avgExecutionTime,
			avgPartitionsScanned:      avgPartitionsScanned,
			avgQueuedOverloadTime:     avgQueuedOverloadTime,
			avgQueuedProvisioningTime: avgQueuedProvisioningTime,
			avgQueuedRepairTime:       avgQueuedRepairTime,
			avgRowsInserted:           avgRowsInserted,
			avgRowsDeleted:            avgRowsDeleted,
			avgRowsProduced:           avgRowsProduced,
			avgRowsUnloaded:           avgRowsUnloaded,
			avgRowsUpdated:            avgRowsUpdated,
			avgTotalElapsedTime:       avgTotalElapsedTime,
		}
		res = append(res, db)
	}
	return &res, nil
}

func (c snowflakeClient) FetchSessionMetrics(ctx context.Context) (*[]sessionMetric, error) {
	rows, err := c.readDB(ctx, sessionMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", sessionMetricsQuery)
		return nil, err
	}

	var res []sessionMetric

	for rows.Next() {
		var userName sql.NullString
		var distinctSessionID int64

		err := rows.Scan(&userName, &distinctSessionID)
		if err != nil {
			return nil, err
		}
		res = append(res, sessionMetric{
			userName:          userName,
			distinctSessionID: distinctSessionID,
		})
	}

	return &res, nil
}

func (c snowflakeClient) FetchSnowpipeMetrics(ctx context.Context) (*[]snowpipeMetric, error) {
	rows, err := c.readDB(ctx, snowpipeMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", snowpipeMetricsQuery)
		return nil, err
	}

	var res []snowpipeMetric

	for rows.Next() {
		var pipeName sql.NullString
		var creditsUsed float64
		var bytesInserted, filesInserted int64

		err := rows.Scan(&pipeName, &creditsUsed, &bytesInserted, &filesInserted)
		if err != nil {
			return nil, err
		}
		res = append(res, snowpipeMetric{
			pipeName:      pipeName,
			creditsUsed:   creditsUsed,
			bytesInserted: bytesInserted,
			filesInserted: filesInserted,
		})
	}

	return &res, nil
}

func (c snowflakeClient) FetchStorageMetrics(ctx context.Context) (*[]storageMetric, error) {
	rows, err := c.readDB(ctx, storageMetricsQuery)
	if err != nil {
		return nil, err
	}

	if rows == nil {
		err = fmt.Errorf("no rows returned by query: %v", storageMetricsQuery)
		return nil, err
	}

	var res []storageMetric

	for rows.Next() {
		var storageBytes, stageBytes, failsafeBytes int64
		err := rows.Scan(&storageBytes, &stageBytes, &failsafeBytes)
		if err != nil {
			return nil, err
		}
		res = append(res, storageMetric{
			storageBytes:  storageBytes,
			stageBytes:    stageBytes,
			failsafeBytes: failsafeBytes,
		})
	}
	return &res, nil
}
