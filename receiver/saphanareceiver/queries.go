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
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

type queryStat struct {
	key                     string
	addIntMetricFunction    func(*sapHanaScraper, pdata.Timestamp, int64, map[string]string)
	addDoubleMetricFunction func(*sapHanaScraper, pdata.Timestamp, float64, map[string]string)
}

func (q *queryStat) collectStat(s *sapHanaScraper, now pdata.Timestamp, row map[string]string) error {
	if val, ok := row[q.key]; ok {
		if q.addIntMetricFunction != nil {
			if i, ok := s.parseInt(q.key, val); ok {
				q.addIntMetricFunction(s, now, i, row)
			} else {
				return fmt.Errorf("unable to parse '%s' as an integer for query key %s", val, q.key)
			}
		} else if q.addDoubleMetricFunction != nil {
			if f, ok := s.parseDouble(q.key, val); ok {
				q.addDoubleMetricFunction(s, now, f, row)
			} else {
				return fmt.Errorf("unable to parse '%s' as a double for query key %s", val, q.key)
			}
		} else {
			return errors.New("incorrectly configured query, either addIntMetricFunction or addDoubleMetricFunction must be provided")
		}
	}
	return nil
}

type monitoringQuery struct {
	query         string
	orderedLabels []string
	orderedStats  []queryStat
}

var queries = []monitoringQuery{
	{
		query:         "SELECT HOST, SUM(CASE WHEN ACTIVE_STATUS = 'YES' THEN 1 ELSE 0 END) AS active_services, SUM(CASE WHEN ACTIVE_STATUS = 'YES' THEN 0 ELSE 1 END) AS inactive_services FROM M_SERVICES GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "active_services",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceCountDataPoint(now, i, row["host"], metadata.AttributeServiceStatus.Active)
				},
			},
			{
				key: "inactive_services",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceCountDataPoint(now, i, row["host"], metadata.AttributeServiceStatus.Inactive)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SUM(CASE WHEN IS_ACTIVE = 'TRUE' THEN 1 ELSE 0 END) AS active_threads, SUM(CASE WHEN IS_ACTIVE = 'TRUE' THEN 0 ELSE 1 END) AS inactive_threads FROM M_SERVICE_THREADS GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "active_threads",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceThreadCountDataPoint(now, i, row["host"], metadata.AttributeThreadStatus.Active)
				},
			},
			{
				key: "inactive_threads",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceThreadCountDataPoint(now, i, row["host"], metadata.AttributeThreadStatus.Inactive)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SUM(MEMORY_SIZE_IN_MAIN) as main, SUM(MEMORY_SIZE_IN_DELTA) as delta FROM M_CS_ALL_COLUMNS GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "main",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, i, row["host"], metadata.AttributeColumnMemoryType.Main)
				},
			},
			{
				key: "delta",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaColumnMemoryUsedDataPoint(now, i, row["host"], metadata.AttributeColumnMemoryType.Delta)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SUM(USED_FIXED_PART_SIZE) fixed, SUM(USED_VARIABLE_PART_SIZE) variable FROM M_RS_TABLES GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "fixed",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaRowStoreMemoryUsedDataPoint(now, i, row["host"], metadata.AttributeRowMemoryType.Fixed)
				},
			},
			{
				key: "variable",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaRowStoreMemoryUsedDataPoint(now, i, row["host"], metadata.AttributeRowMemoryType.Variable)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, COMPONENT, sum(USED_MEMORY_SIZE) used_mem_size FROM M_SERVICE_COMPONENT_MEMORY GROUP BY HOST, COMPONENT",
		orderedLabels: []string{"host", "component"},
		orderedStats: []queryStat{
			{
				key: "used_mem_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaComponentMemoryUsedDataPoint(now, i, row["host"], row["component"])
				},
			},
		},
	},
	{
		query:         "SELECT HOST, CONNECTION_STATUS, COUNT(*) AS connections FROM M_CONNECTIONS WHERE CONNECTION_STATUS != '' GROUP BY HOST, CONNECTION_STATUS",
		orderedLabels: []string{"host", "connection_status"},
		orderedStats: []queryStat{
			{
				key: "connections",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaConnectionCountDataPoint(now, i, row["host"], strings.ToLower(row["connection_status"]))
				},
			},
		},
	},
	{
		query:         "SELECT seconds_between(CURRENT_TIMESTAMP, UTC_START_TIME) age FROM M_BACKUP_CATALOG WHERE STATE_NAME = 'successful' ORDER BY UTC_START_TIME DESC LIMIT 1",
		orderedLabels: []string{},
		orderedStats: []queryStat{
			{
				key: "age",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaBackupLatestDataPoint(now, i)
				},
			},
		},
	},
	{
		query:         "SSELECT HOST, SYSTEM_ID, DATABASE_NAME, seconds_between(START_TIME, CURRENT_TIMESTAMP) age  FROM M_DATABASE",
		orderedLabels: []string{"host", "system", "database"},
		orderedStats: []queryStat{
			{
				key: "age",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaUptimeDataPoint(now, i, row["host"], row["system"], row["database"])
				},
			},
		},
	},
	{
		query:         "SELECT ALERT_RATING, COUNT(*) AS alerts FROM _SYS_STATISTICS.STATISTICS_CURRENT_ALERTS GROUP BY ALERT_RATING",
		orderedLabels: []string{"alert_rating"},
		orderedStats: []queryStat{
			{
				key: "alerts",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaAlertCountDataPoint(now, i, row["alert_rating"])
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SUM(UPDATE_TRANSACTION_COUNT) updates, SUM(COMMIT_COUNT) commits, SUM(ROLLBACK_COUNT) rollbacks FROM M_WORKLOAD GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "updates",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaTransactionCountDataPoint(now, i, row["host"], metadata.AttributeTransactionType.Update)
				},
			},
			{
				key: "commits",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaTransactionCountDataPoint(now, i, row["host"], metadata.AttributeTransactionType.Commit)
				},
			},
			{
				key: "rollbacks",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaTransactionCountDataPoint(now, i, row["host"], metadata.AttributeTransactionType.Rollback)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, COUNT(*) blocks FROM m_blocked_transactions GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "blocks",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaTransactionBlockedDataPoint(now, i, row["host"])
				},
			},
		},
	},
	{
		query:         "SELECT HOST, \"PATH\", USAGE_TYPE, TOTAL_SIZE-USED_SIZE free_size, USED_SIZE FROM M_DISKS",
		orderedLabels: []string{"host", "path", "usage_type"},
		orderedStats: []queryStat{
			{
				key: "free_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaDiskSizeCurrentDataPoint(now, i, row["host"], row["path"], row["usage_type"], metadata.AttributeDiskStateUsedFree.Free)
				},
			},
			{
				key: "used_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaDiskSizeCurrentDataPoint(now, i, row["host"], row["path"], row["usage_type"], metadata.AttributeDiskStateUsedFree.Used)
				},
			},
		},
	},
	{
		query:         "SELECT SYSTEM_ID, PRODUCT_NAME, PRODUCT_LIMIT, PRODUCT_USAGE, seconds_between(CURRENT_TIMESTAMP, EXPIRATION_DATE) expiration FROM M_LICENSES",
		orderedLabels: []string{"system", "product"},
		orderedStats: []queryStat{
			{
				key: "limit",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaLicenseLimitDataPoint(now, i, row["system"], row["product"])
				},
			},
			{
				key: "usage",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaLicensePeakDataPoint(now, i, row["system"], row["product"])
				},
			},
			{
				key: "expiration",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaLicenseExpirationTimeDataPoint(now, i, row["system"], row["product"])
				},
			},
		},
	},
	{
		query:         "SELECT HOST, PORT, SECONDARY_HOST, REPLICATION_MODE, BACKLOG_SIZE, BACKLOG_TIME, TO_DECIMAL(IFNULL(MAP(SHIPPED_LOG_BUFFERS_COUNT, 0, 0, SHIPPED_LOG_BUFFERS_DURATION / SHIPPED_LOG_BUFFERS_COUNT), 0), 10, 2) avg_replication_time FROM M_SERVICE_REPLICATION",
		orderedLabels: []string{"host", "port", "secondary", "mode"},
		orderedStats: []queryStat{
			{
				key: "backlog_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaReplicationBacklogSizeDataPoint(now, i, row["host"], row["secondary"], row["port"], row["mode"])
				},
			},
			{
				key: "backlog_time",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaReplicationBacklogTimeDataPoint(now, i, row["host"], row["secondary"], row["port"], row["mode"])
				},
			},
			{
				key: "average_time",
				addDoubleMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, f float64, row map[string]string) {
					s.mb.RecordSaphanaReplicationAverageTimeDataPoint(now, f, row["host"], row["secondary"], row["port"], row["mode"])
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SUM(FINISHED_NON_INTERNAL_REQUEST_COUNT) \"external\", SUM(ALL_FINISHED_REQUEST_COUNT-FINISHED_NON_INTERNAL_REQUEST_COUNT) internal, SUM(ACTIVE_REQUEST_COUNT) active, SUM(PENDING_REQUEST_COUNT) pending, TO_DECIMAL(AVG(RESPONSE_TIME), 10, 2) avg_time FROM M_SERVICE_STATISTICS WHERE ACTIVE_REQUEST_COUNT > -1 GROUP BY HOST",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "external",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaNetworkRequestFinishedCountDataPoint(now, i, row["host"], metadata.AttributeInternalExternalRequestType.External)
				},
			},
			{
				key: "internal",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaNetworkRequestFinishedCountDataPoint(now, i, row["host"], metadata.AttributeInternalExternalRequestType.Internal)
				},
			},
			{
				key: "active",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaNetworkRequestCountDataPoint(now, i, row["host"], metadata.AttributeActivePendingRequestState.Active)
				},
			},
			{
				key: "pending",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaNetworkRequestCountDataPoint(now, i, row["host"], metadata.AttributeActivePendingRequestState.Pending)
				},
			},
			{
				key: "avg_time",
				addDoubleMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, f float64, row map[string]string) {
					s.mb.RecordSaphanaNetworkRequestAverageTimeDataPoint(now, f, row["host"])
				},
			},
		},
	},
	//SELECT HOST, SUM(FINISHED_NON_INTERNAL_REQUEST_COUNT) "external", SUM(ALL_FINISHED_REQUEST_COUNT-FINISHED_NON_INTERNAL_REQUEST_COUNT) internal, SUM(ACTIVE_REQUEST_COUNT) active, SUM(PENDING_REQUEST_COUNT) pending FROM M_SERVICE_STATISTICS WHERE ACTIVE_REQUEST_COUNT > -1 GROUP BY HOST
	{
		query:         "SELECT HOST, \"PATH\", \"TYPE\", SUM(TOTAL_READS) \"reads\", SUM(TOTAL_WRITES) writes, SUM(TOTAL_READ_SIZE) read_size, SUM(TOTAL_WRITE_SIZE) write_size, SUM(TOTAL_READ_TIME) read_time, SUM(TOTAL_WRITE_TIME) write_time FROM M_VOLUME_IO_TOTAL_STATISTICS GROUP BY HOST, \"PATH\", \"TYPE\"",
		orderedLabels: []string{"host", "path", "type"},
		orderedStats: []queryStat{
			{
				key: "reads",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaVolumeOperationCountDataPoint(now, i, row["host"], row["path"], row["type"], metadata.AttributeVolumeOperationType.Read)
				},
			},
			{
				key: "writes",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaVolumeOperationCountDataPoint(now, i, row["host"], row["path"], row["type"], metadata.AttributeVolumeOperationType.Write)
				},
			},
			{
				key: "read_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaVolumeOperationSizeDataPoint(now, i, row["host"], row["path"], row["type"], metadata.AttributeVolumeOperationType.Read)
				},
			},
			{
				key: "write_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaVolumeOperationSizeDataPoint(now, i, row["host"], row["path"], row["type"], metadata.AttributeVolumeOperationType.Write)
				},
			},
			{
				key: "read_time",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaVolumeOperationTimeDataPoint(now, i, row["host"], row["path"], row["type"], metadata.AttributeVolumeOperationType.Read)
				},
			},
			{
				key: "write_time",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaVolumeOperationTimeDataPoint(now, i, row["host"], row["path"], row["type"], metadata.AttributeVolumeOperationType.Write)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SERVICE_NAME, LOGICAL_MEMORY_SIZE, PHYSICAL_MEMORY_SIZE, CODE_SIZE, STACK_SIZE, HEAP_MEMORY_ALLOCATED_SIZE-HEAP_MEMORY_USED_SIZE heap_free, HEAP_MEMORY_USED_SIZE, SHARED_MEMORY_ALLOCATED_SIZE-SHARED_MEMORY_USED_SIZE shared_free, SHARED_MEMORY_USED_SIZE, COMPACTORS_ALLOCATED_SIZE, COMPACTORS_FREEABLE_SIZE, ALLOCATION_LIMIT, EFFECTIVE_ALLOCATION_LIMIT FROM M_SERVICE_MEMORY",
		orderedLabels: []string{"host", "service"},
		orderedStats: []queryStat{
			{
				key: "logical_used",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryUsedDataPoint(now, i, row["host"], row["service"], metadata.AttributeServiceMemoryUsedType.Logical)
				},
			},
			{
				key: "physical_used",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryUsedDataPoint(now, i, row["host"], row["service"], metadata.AttributeServiceMemoryUsedType.Physical)
				},
			},
			{
				key: "code_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceCodeSizeDataPoint(now, i, row["host"], row["service"])
				},
			},
			{
				key: "stack_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceStackSizeDataPoint(now, i, row["host"], row["service"])
				},
			},
			{
				key: "heap_free",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryHeapCurrentDataPoint(now, i, row["host"], row["service"], metadata.AttributeMemoryStateUsedFree.Free)
				},
			},
			{
				key: "heap_used",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryHeapCurrentDataPoint(now, i, row["host"], row["service"], metadata.AttributeMemoryStateUsedFree.Used)
				},
			},
			{
				key: "shared_free",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemorySharedCurrentDataPoint(now, i, row["host"], row["service"], metadata.AttributeMemoryStateUsedFree.Free)
				},
			},
			{
				key: "shared_used",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemorySharedCurrentDataPoint(now, i, row["host"], row["service"], metadata.AttributeMemoryStateUsedFree.Used)
				},
			},
			{
				key: "compactors_allocated",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryCompactorsAllocatedDataPoint(now, i, row["host"], row["service"])
				},
			},
			{
				key: "compactors_freeable",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryCompactorsFreeableDataPoint(now, i, row["host"], row["service"])
				},
			},
			{
				key: "allocation_limit",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryLimitDataPoint(now, i, row["host"], row["service"])
				},
			},
			{
				key: "effective_limit",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaServiceMemoryEffectiveLimitDataPoint(now, i, row["host"], row["service"])
				},
			},
		},
	},
	{
		query:         "SELECT HOST, SCHEMA_NAME, SUM(ESTIMATED_MAX_MEMORY_SIZE_IN_TOTAL) estimated_max, SUM(LAST_COMPRESSED_RECORD_COUNT) last_compressed, SUM(READ_COUNT) \"reads\", SUM(WRITE_COUNT) writes, SUM(MERGE_COUNT) merges, SUM(MEMORY_SIZE_IN_MAIN) mem_main, SUM(MEMORY_SIZE_IN_DELTA) mem_delta, SUM(MEMORY_SIZE_IN_HISTORY_MAIN) mem_hist_main, SUM(MEMORY_SIZE_IN_HISTORY_DELTA) mem_hist_delta, SUM(RAW_RECORD_COUNT_IN_MAIN) records_main, SUM(RAW_RECORD_COUNT_IN_DELTA) records_delta, SUM(RAW_RECORD_COUNT_IN_HISTORY_MAIN) records_hist_main, SUM(RAW_RECORD_COUNT_IN_HISTORY_DELTA) records_hist_delta FROM M_CS_TABLES GROUP BY HOST, SCHEMA_NAME",
		orderedLabels: []string{"host", "schema"},
		orderedStats: []queryStat{
			{
				key: "estimated_max",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaMemoryUsedMaxDataPoint(now, i, row["host"], row["schema"])
				},
			},
			{
				key: "last_compressed",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaRecordCompressedCountDataPoint(now, i, row["host"], row["schema"])
				},
			},
			{
				key: "reads",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaOperationCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaOperationType.Read)
				},
			},
			{
				key: "writes",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaOperationCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaOperationType.Write)
				},
			},
			{
				key: "merges",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaOperationCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaOperationType.Merge)
				},
			},
			{
				key: "mem_main",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaMemoryType.Main)
				},
			},
			{
				key: "mem_delta",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaMemoryType.Delta)
				},
			},
			{
				key: "mem_history_main",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaMemoryType.HistoryMain)
				},
			},
			{
				key: "mem_history_delta",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaMemoryType.HistoryDelta)
				},
			},
			{
				key: "records_main",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaRecordCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaRecordType.Main)
				},
			},
			{
				key: "records_delta",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaRecordCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaRecordType.Delta)
				},
			},
			{
				key: "records_history_main",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaRecordCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaRecordType.HistoryMain)
				},
			},
			{
				key: "records_history_delta",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaSchemaRecordCountDataPoint(now, i, row["host"], row["schema"], metadata.AttributeSchemaRecordType.HistoryDelta)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, FREE_PHYSICAL_MEMORY, USED_PHYSICAL_MEMORY, FREE_SWAP_SPACE, USED_SWAP_SPACE, INSTANCE_TOTAL_MEMORY_USED_SIZE, INSTANCE_TOTAL_MEMORY_PEAK_USED_SIZE, INSTANCE_TOTAL_MEMORY_ALLOCATED_SIZE-INSTANCE_TOTAL_MEMORY_USED_SIZE total_free, INSTANCE_CODE_SIZE, INSTANCE_SHARED_MEMORY_ALLOCATED_SIZE, TOTAL_CPU_USER_TIME, TOTAL_CPU_SYSTEM_TIME, TOTAL_CPU_WIO_TIME, TOTAL_CPU_IDLE_TIME FROM M_HOST_RESOURCE_UTILIZATION",
		orderedLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "free_physical_memory",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaHostMemoryCurrentDataPoint(now, i, row["host"], metadata.AttributeMemoryStateUsedFree.Free)
				},
			},
			{
				key: "used_physical_memory",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaHostMemoryCurrentDataPoint(now, i, row["host"], metadata.AttributeMemoryStateUsedFree.Used)
				},
			},
			{
				key: "free_swap_space",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaHostSwapCurrentDataPoint(now, i, row["host"], metadata.AttributeHostSwapState.Free)
				},
			},
			{
				key: "used_swap_space",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaHostSwapCurrentDataPoint(now, i, row["host"], metadata.AttributeHostSwapState.Used)
				},
			},
			{
				key: "instance_total_used",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaInstanceMemoryCurrentDataPoint(now, i, row["host"], metadata.AttributeMemoryStateUsedFree.Used)
				},
			},
			{
				key: "instance_total_used_peak",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaInstanceMemoryUsedPeakDataPoint(now, i, row["host"])
				},
			},
			{
				key: "instance_total_free",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaInstanceMemoryCurrentDataPoint(now, i, row["host"], metadata.AttributeMemoryStateUsedFree.Free)
				},
			},
			{
				key: "instance_code_size",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaInstanceCodeSizeDataPoint(now, i, row["host"])
				},
			},
			{
				key: "instance_shared_memory_allocated",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaInstanceMemorySharedAllocatedDataPoint(now, i, row["host"])
				},
			},
			{
				key: "cpu_user",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaCPUUsedDataPoint(now, i, row["host"], metadata.AttributeCPUType.User)
				},
			},
			{
				key: "cpu_system",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaCPUUsedDataPoint(now, i, row["host"], metadata.AttributeCPUType.System)
				},
			},
			{
				key: "cpu_io_wait",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaCPUUsedDataPoint(now, i, row["host"], metadata.AttributeCPUType.IoWait)
				},
			},
			{
				key: "cpu_idle",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaCPUUsedDataPoint(now, i, row["host"], metadata.AttributeCPUType.Idle)
				},
			},
		},
	},
	{
		query:         "SELECT HOST, CONNECTION_STATUS, COUNT(*) AS connections FROM M_CONNECTIONS WHERE CONNECTION_STATUS != '' GROUP BY HOST, CONNECTION_STATUS",
		orderedLabels: []string{"host", "connection_status"},
		orderedStats: []queryStat{
			{
				key: "connections",
				addIntMetricFunction: func(s *sapHanaScraper, now pdata.Timestamp, i int64, row map[string]string) {
					s.mb.RecordSaphanaConnectionCountDataPoint(now, i, row["host"], strings.ToLower(row["connection_status"]))
				},
			},
		},
	},
}

func (m *monitoringQuery) CollectMetrics(s *sapHanaScraper, ctx context.Context, client client, now pdata.Timestamp) error {
	if rows, err := client.collectDataFromQuery(ctx, m); err != nil {
		return err
	} else {
		for _, data := range rows {
			for _, stat := range m.orderedStats {
				stat.collectStat(s, now, data)
			}
		}
	}

	return nil
}
