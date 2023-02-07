// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
