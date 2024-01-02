// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import "database/sql"

// each query returns columns which serialize into these data structures
// these are consumed by the scraper to create and emit metrics

// billing metrics query
type billingMetric struct {
	serviceType                  sql.NullString
	serviceName                  sql.NullString
	totalCloudService            float64
	totalCredits                 float64
	totalVirtualWarehouseCredits float64
}

// warehouse billing query
type whBillingMetric struct {
	warehouseName         sql.NullString
	totalCloudService     float64
	totalCredit           float64
	totalVirtualWarehouse float64
}

// login metrics query
type loginMetric struct {
	userName           sql.NullString
	errorMessage       sql.NullString
	reportedClientType sql.NullString
	isSuccess          sql.NullString
	loginsTotal        int64
}

// high level low dimensionality query
type hlQueryMetric struct {
	warehouseName           sql.NullString
	avgQueryExecuted        float64
	avgQueryBlocked         float64
	avgQueryQueuedOverload  float64
	avgQueryQueuedProvision float64
}

// db metrics query
type dbMetric struct {
	attributes                dbMetricAttributes
	databaseQueryCount        int64
	avgBytesScanned           float64
	avgBytesDeleted           float64
	avgBytesSpilledRemote     float64
	avgBytesSpilledLocal      float64
	avgBytesWritten           float64
	avgCompilationTime        float64
	avgDataScannedCache       float64
	avgExecutionTime          float64
	avgPartitionsScanned      float64
	avgQueuedOverloadTime     float64
	avgQueuedProvisioningTime float64
	avgQueuedRepairTime       float64
	avgRowsInserted           float64
	avgRowsDeleted            float64
	avgRowsProduced           float64
	avgRowsUnloaded           float64
	avgRowsUpdated            float64
	avgTotalElapsedTime       float64
}

type dbMetricAttributes struct {
	userName        sql.NullString
	schemaName      sql.NullString
	executionStatus sql.NullString
	errorMessage    sql.NullString
	queryType       sql.NullString
	warehouseName   sql.NullString
	databaseName    sql.NullString
	warehouseSize   sql.NullString
}

type sessionMetric struct {
	userName          sql.NullString
	distinctSessionID int64
}

type snowpipeMetric struct {
	pipeName      sql.NullString
	creditsUsed   float64
	bytesInserted int64
	filesInserted int64
}

type storageMetric struct {
	storageBytes  int64
	stageBytes    int64
	failsafeBytes int64
}
