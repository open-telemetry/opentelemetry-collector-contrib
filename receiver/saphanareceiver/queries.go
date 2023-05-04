// Copyright The OpenTelemetry Authors
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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

type queryStat struct {
	key               string
	addMetricFunction func(*metadata.MetricsBuilder, pcommon.Timestamp, string, map[string]string) error
}

func (q *queryStat) collectStat(s *sapHanaScraper, m *monitoringQuery, now pcommon.Timestamp,
	row map[string]string) error {
	if val, ok := row[q.key]; ok {
		resourceAttributes := map[string]string{}
		for _, attr := range m.orderedResourceLabels {
			attrValue, ok := row[attr]
			if !ok {
				return fmt.Errorf("unable to parse metric for key %s, missing resource attribute '%s'", q.key, attr)
			}
			resourceAttributes[attr] = attrValue
		}
		mb, err := s.getMetricsBuilder(resourceAttributes)
		if err != nil {
			return fmt.Errorf("unable to parse metric for key %s: %w", q.key, err)
		}

		if q.addMetricFunction != nil {
			if err = q.addMetricFunction(mb, now, val, row); err != nil {
				return fmt.Errorf("failed to record metric for key %s: %w", q.key, err)
			}
		} else {
			return errors.New("incorrectly configured query, addMetricFunction must be provided")
		}
	}
	return nil
}

type monitoringQuery struct {
	query                 string
	orderedResourceLabels []string
	orderedMetricLabels   []string
	orderedStats          []queryStat
	Enabled               func(c *Config) bool
}

var queries = []monitoringQuery{
	{
		query:                 "SELECT HOST, SUM(CASE WHEN ACTIVE_STATUS = 'YES' THEN 1 ELSE 0 END) AS active_services, SUM(CASE WHEN ACTIVE_STATUS = 'YES' THEN 0 ELSE 1 END) AS inactive_services FROM SYS.M_SERVICES GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "active_services",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceCountDataPoint(now, val, metadata.AttributeServiceStatusActive)
				},
			},
			{
				key: "inactive_services",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceCountDataPoint(now, val, metadata.AttributeServiceStatusInactive)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaServiceCount.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SUM(CASE WHEN IS_ACTIVE = 'TRUE' THEN 1 ELSE 0 END) AS active_threads, SUM(CASE WHEN IS_ACTIVE = 'TRUE' THEN 0 ELSE 1 END) AS inactive_threads FROM SYS.M_SERVICE_THREADS GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "active_threads",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceThreadCountDataPoint(now, val, metadata.AttributeThreadStatusActive)
				},
			},
			{
				key: "inactive_threads",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceThreadCountDataPoint(now, val, metadata.AttributeThreadStatusInactive)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaServiceThreadCount.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SUM(MAIN_MEMORY_SIZE_IN_DATA) AS \"mem_main_data\", SUM(MAIN_MEMORY_SIZE_IN_DICT) AS \"mem_main_dict\", SUM(MAIN_MEMORY_SIZE_IN_INDEX) AS \"mem_main_index\", SUM(MAIN_MEMORY_SIZE_IN_MISC) AS \"mem_main_misc\", SUM(DELTA_MEMORY_SIZE_IN_DATA) AS \"mem_delta_data\", SUM(DELTA_MEMORY_SIZE_IN_DICT) AS \"mem_delta_dict\", SUM(DELTA_MEMORY_SIZE_IN_INDEX) AS \"mem_delta_index\", SUM(DELTA_MEMORY_SIZE_IN_MISC) AS \"mem_delta_misc\" FROM M_CS_ALL_COLUMNS GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "main_data",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeMain, metadata.AttributeColumnMemorySubtypeData)
				},
			},
			{
				key: "main_dict",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeMain, metadata.AttributeColumnMemorySubtypeDict)
				},
			},
			{
				key: "main_index",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeMain, metadata.AttributeColumnMemorySubtypeIndex)
				},
			},
			{
				key: "main_misc",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeMain, metadata.AttributeColumnMemorySubtypeMisc)
				},
			},
			{
				key: "delta_data",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeDelta, metadata.AttributeColumnMemorySubtypeData)
				},
			},
			{
				key: "delta_dict",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeDelta, metadata.AttributeColumnMemorySubtypeDict)
				},
			},
			{
				key: "delta_index",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeDelta, metadata.AttributeColumnMemorySubtypeIndex)
				},
			},
			{
				key: "delta_misc",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaColumnMemoryUsedDataPoint(now, val, metadata.AttributeColumnMemoryTypeDelta, metadata.AttributeColumnMemorySubtypeMisc)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaColumnMemoryUsed.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SUM(USED_FIXED_PART_SIZE) fixed, SUM(USED_VARIABLE_PART_SIZE) variable FROM SYS.M_RS_TABLES GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "fixed",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaRowStoreMemoryUsedDataPoint(now, val, metadata.AttributeRowMemoryTypeFixed)
				},
			},
			{
				key: "variable",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaRowStoreMemoryUsedDataPoint(now, val, metadata.AttributeRowMemoryTypeVariable)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaRowStoreMemoryUsed.Enabled
		},
	},
	{
		query:                 "SELECT HOST, COMPONENT, sum(USED_MEMORY_SIZE) used_mem_size FROM SYS.M_SERVICE_COMPONENT_MEMORY GROUP BY HOST, COMPONENT",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"component"},
		orderedStats: []queryStat{
			{
				key: "used_mem_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaComponentMemoryUsedDataPoint(now, val, row["component"])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaComponentMemoryUsed.Enabled
		},
	},
	{
		query:                 "SELECT HOST, CONNECTION_STATUS, COUNT(*) AS connections FROM SYS.M_CONNECTIONS WHERE CONNECTION_STATUS != '' GROUP BY HOST, CONNECTION_STATUS",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"connection_status"},
		orderedStats: []queryStat{
			{
				key: "connections",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaConnectionCountDataPoint(now, val,
						metadata.MapAttributeConnectionStatus[strings.ToLower(row["connection_status"])])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaConnectionCount.Enabled
		},
	},
	{
		query:               "SELECT seconds_between(CURRENT_TIMESTAMP, UTC_START_TIME) age FROM SYS.M_BACKUP_CATALOG WHERE STATE_NAME = 'successful' ORDER BY UTC_START_TIME DESC LIMIT 1",
		orderedMetricLabels: []string{},
		orderedStats: []queryStat{
			{
				key: "age",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaBackupLatestDataPoint(now, val)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaBackupLatest.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SYSTEM_ID, DATABASE_NAME, seconds_between(START_TIME, CURRENT_TIMESTAMP) age FROM SYS.M_DATABASE",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"system", "database"},
		orderedStats: []queryStat{
			{
				key: "age",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaUptimeDataPoint(now, val, row["system"], row["database"])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaUptime.Enabled
		},
	},
	{
		query:               "SELECT ALERT_RATING, COUNT(*) AS alerts FROM _SYS_STATISTICS.STATISTICS_CURRENT_ALERTS GROUP BY ALERT_RATING",
		orderedMetricLabels: []string{"alert_rating"},
		orderedStats: []queryStat{
			{
				key: "alerts",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaAlertCountDataPoint(now, val, row["alert_rating"])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaAlertCount.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SUM(UPDATE_TRANSACTION_COUNT) updates, SUM(COMMIT_COUNT) commits, SUM(ROLLBACK_COUNT) rollbacks FROM SYS.M_WORKLOAD GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "updates",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaTransactionCountDataPoint(now, val, metadata.AttributeTransactionTypeUpdate)
				},
			},
			{
				key: "commits",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaTransactionCountDataPoint(now, val, metadata.AttributeTransactionTypeCommit)
				},
			},
			{
				key: "rollbacks",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaTransactionCountDataPoint(now, val, metadata.AttributeTransactionTypeRollback)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaTransactionCount.Enabled
		},
	},
	{
		query:                 "SELECT HOST, COUNT(*) blocks FROM SYS.M_BLOCKED_TRANSACTIONS GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "blocks",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaTransactionBlockedDataPoint(now, val)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaTransactionBlocked.Enabled
		},
	},
	{
		query:                 "SELECT HOST, \"PATH\", USAGE_TYPE, TOTAL_SIZE-USED_SIZE free_size, USED_SIZE FROM SYS.M_DISKS",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"path", "usage_type"},
		orderedStats: []queryStat{
			{
				key: "free_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaDiskSizeCurrentDataPoint(now, val, row["path"], row["usage_type"], metadata.AttributeDiskStateUsedFreeFree)
				},
			},
			{
				key: "used_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaDiskSizeCurrentDataPoint(now, val, row["path"], row["usage_type"], metadata.AttributeDiskStateUsedFreeUsed)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaDiskSizeCurrent.Enabled
		},
	},
	{
		query:               "SELECT SYSTEM_ID, PRODUCT_NAME, PRODUCT_LIMIT, PRODUCT_USAGE, seconds_between(CURRENT_TIMESTAMP, EXPIRATION_DATE) expiration FROM SYS.M_LICENSES",
		orderedMetricLabels: []string{"system", "product"},
		orderedStats: []queryStat{
			{
				key: "limit",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaLicenseLimitDataPoint(now, val, row["system"], row["product"])
				},
			},
			{
				key: "usage",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaLicensePeakDataPoint(now, val, row["system"], row["product"])
				},
			},
			{
				key: "expiration",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaLicenseExpirationTimeDataPoint(now, val, row["system"], row["product"])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaLicenseExpirationTime.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaLicenseLimit.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaLicensePeak.Enabled
		},
	},
	{
		query:               "SELECT HOST, PORT, SECONDARY_HOST, REPLICATION_MODE, BACKLOG_SIZE, BACKLOG_TIME, TO_VARCHAR(TO_DECIMAL(IFNULL(MAP(SHIPPED_LOG_BUFFERS_COUNT, 0, 0, SHIPPED_LOG_BUFFERS_DURATION / SHIPPED_LOG_BUFFERS_COUNT), 0), 10, 2)) avg_replication_time FROM SYS.M_SERVICE_REPLICATION",
		orderedMetricLabels: []string{"host", "port", "secondary", "mode"},
		orderedStats: []queryStat{
			{
				key: "backlog_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaReplicationBacklogSizeDataPoint(now, val, row["host"], row["secondary"], row["port"], row["mode"])
				},
			},
			{
				key: "backlog_time",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaReplicationBacklogTimeDataPoint(now, val, row["host"], row["secondary"], row["port"], row["mode"])
				},
			},
			{
				key: "average_time",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaReplicationAverageTimeDataPoint(now, val, row["host"], row["secondary"], row["port"], row["mode"])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaReplicationAverageTime.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaReplicationBacklogSize.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaReplicationBacklogTime.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SUM(FINISHED_NON_INTERNAL_REQUEST_COUNT) \"external\", SUM(ALL_FINISHED_REQUEST_COUNT-FINISHED_NON_INTERNAL_REQUEST_COUNT) internal, SUM(ACTIVE_REQUEST_COUNT) active, SUM(PENDING_REQUEST_COUNT) pending, TO_VARCHAR(TO_DECIMAL(AVG(RESPONSE_TIME), 10, 2)) avg_time FROM SYS.M_SERVICE_STATISTICS WHERE ACTIVE_REQUEST_COUNT > -1 GROUP BY HOST",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "external",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaNetworkRequestFinishedCountDataPoint(now, val, metadata.AttributeInternalExternalRequestTypeExternal)
				},
			},
			{
				key: "internal",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaNetworkRequestFinishedCountDataPoint(now, val, metadata.AttributeInternalExternalRequestTypeInternal)
				},
			},
			{
				key: "active",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaNetworkRequestCountDataPoint(now, val, metadata.AttributeActivePendingRequestStateActive)
				},
			},
			{
				key: "pending",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaNetworkRequestCountDataPoint(now, val, metadata.AttributeActivePendingRequestStatePending)
				},
			},
			{
				key: "avg_time",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaNetworkRequestAverageTimeDataPoint(now, val)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaNetworkRequestFinishedCount.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaNetworkRequestCount.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaNetworkRequestAverageTime.Enabled
		},
	},
	{
		query:                 "SELECT HOST, \"PATH\", \"TYPE\", SUM(TOTAL_READS) \"reads\", SUM(TOTAL_WRITES) writes, SUM(TOTAL_READ_SIZE) read_size, SUM(TOTAL_WRITE_SIZE) write_size, SUM(TOTAL_READ_TIME) read_time, SUM(TOTAL_WRITE_TIME) write_time FROM SYS.M_VOLUME_IO_TOTAL_STATISTICS GROUP BY HOST, \"PATH\", \"TYPE\"",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"path", "type"},
		orderedStats: []queryStat{
			{
				key: "reads",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaVolumeOperationCountDataPoint(now, val, row["path"], row["type"], metadata.AttributeVolumeOperationTypeRead)
				},
			},
			{
				key: "writes",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaVolumeOperationCountDataPoint(now, val, row["path"], row["type"], metadata.AttributeVolumeOperationTypeWrite)
				},
			},
			{
				key: "read_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaVolumeOperationSizeDataPoint(now, val, row["path"], row["type"], metadata.AttributeVolumeOperationTypeRead)
				},
			},
			{
				key: "write_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaVolumeOperationSizeDataPoint(now, val, row["path"], row["type"], metadata.AttributeVolumeOperationTypeWrite)
				},
			},
			{
				key: "read_time",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaVolumeOperationTimeDataPoint(now, val, row["path"], row["type"], metadata.AttributeVolumeOperationTypeRead)
				},
			},
			{
				key: "write_time",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaVolumeOperationTimeDataPoint(now, val, row["path"], row["type"], metadata.AttributeVolumeOperationTypeWrite)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaVolumeOperationCount.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaVolumeOperationSize.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaVolumeOperationTime.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SERVICE_NAME, LOGICAL_MEMORY_SIZE, PHYSICAL_MEMORY_SIZE, CODE_SIZE, STACK_SIZE, HEAP_MEMORY_ALLOCATED_SIZE-HEAP_MEMORY_USED_SIZE heap_free, HEAP_MEMORY_USED_SIZE, SHARED_MEMORY_ALLOCATED_SIZE-SHARED_MEMORY_USED_SIZE shared_free, SHARED_MEMORY_USED_SIZE, COMPACTORS_ALLOCATED_SIZE, COMPACTORS_FREEABLE_SIZE, ALLOCATION_LIMIT, EFFECTIVE_ALLOCATION_LIMIT FROM SYS.M_SERVICE_MEMORY",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"service"},
		orderedStats: []queryStat{
			{
				key: "logical_used",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryUsedDataPoint(now, val, row["service"], metadata.AttributeServiceMemoryUsedTypeLogical)
				},
			},
			{
				key: "physical_used",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryUsedDataPoint(now, val, row["service"], metadata.AttributeServiceMemoryUsedTypePhysical)
				},
			},
			{
				key: "code_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceCodeSizeDataPoint(now, val, row["service"])
				},
			},
			{
				key: "stack_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceStackSizeDataPoint(now, val, row["service"])
				},
			},
			{
				key: "heap_free",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryHeapCurrentDataPoint(now, val, row["service"], metadata.AttributeMemoryStateUsedFreeFree)
				},
			},
			{
				key: "heap_used",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryHeapCurrentDataPoint(now, val, row["service"], metadata.AttributeMemoryStateUsedFreeUsed)
				},
			},
			{
				key: "shared_free",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemorySharedCurrentDataPoint(now, val, row["service"], metadata.AttributeMemoryStateUsedFreeFree)
				},
			},
			{
				key: "shared_used",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemorySharedCurrentDataPoint(now, val, row["service"], metadata.AttributeMemoryStateUsedFreeUsed)
				},
			},
			{
				key: "compactors_allocated",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryCompactorsAllocatedDataPoint(now, val, row["service"])
				},
			},
			{
				key: "compactors_freeable",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryCompactorsFreeableDataPoint(now, val, row["service"])
				},
			},
			{
				key: "allocation_limit",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryLimitDataPoint(now, val, row["service"])
				},
			},
			{
				key: "effective_limit",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaServiceMemoryEffectiveLimitDataPoint(now, val, row["service"])
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryUsed.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceCodeSize.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceStackSize.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryHeapCurrent.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceMemorySharedCurrent.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryCompactorsAllocated.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryCompactorsFreeable.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryLimit.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaServiceMemoryEffectiveLimit.Enabled
		},
	},
	{
		query:                 "SELECT HOST, SCHEMA_NAME, SUM(ESTIMATED_MAX_MEMORY_SIZE_IN_TOTAL) estimated_max, SUM(LAST_COMPRESSED_RECORD_COUNT) last_compressed, SUM(READ_COUNT) \"reads\", SUM(WRITE_COUNT) writes, SUM(MERGE_COUNT) merges, SUM(MEMORY_SIZE_IN_MAIN) mem_main, SUM(MEMORY_SIZE_IN_DELTA) mem_delta, SUM(MEMORY_SIZE_IN_HISTORY_MAIN) mem_hist_main, SUM(MEMORY_SIZE_IN_HISTORY_DELTA) mem_hist_delta, SUM(RAW_RECORD_COUNT_IN_MAIN) records_main, SUM(RAW_RECORD_COUNT_IN_DELTA) records_delta, SUM(RAW_RECORD_COUNT_IN_HISTORY_MAIN) records_hist_main, SUM(RAW_RECORD_COUNT_IN_HISTORY_DELTA) records_hist_delta FROM SYS.M_CS_TABLES GROUP BY HOST, SCHEMA_NAME",
		orderedResourceLabels: []string{"host"},
		orderedMetricLabels:   []string{"schema"},
		orderedStats: []queryStat{
			{
				key: "estimated_max",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaMemoryUsedMaxDataPoint(now, val, row["schema"])
				},
			},
			{
				key: "last_compressed",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaRecordCompressedCountDataPoint(now, val, row["schema"])
				},
			},
			{
				key: "reads",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaOperationCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaOperationTypeRead)
				},
			},
			{
				key: "writes",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaOperationCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaOperationTypeWrite)
				},
			},
			{
				key: "merges",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaOperationCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaOperationTypeMerge)
				},
			},
			{
				key: "mem_main",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, val, row["schema"], metadata.AttributeSchemaMemoryTypeMain)
				},
			},
			{
				key: "mem_delta",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, val, row["schema"], metadata.AttributeSchemaMemoryTypeDelta)
				},
			},
			{
				key: "mem_history_main",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, val, row["schema"], metadata.AttributeSchemaMemoryTypeHistoryMain)
				},
			},
			{
				key: "mem_history_delta",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaMemoryUsedCurrentDataPoint(now, val, row["schema"], metadata.AttributeSchemaMemoryTypeHistoryDelta)
				},
			},
			{
				key: "records_main",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaRecordCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaRecordTypeMain)
				},
			},
			{
				key: "records_delta",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaRecordCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaRecordTypeDelta)
				},
			},
			{
				key: "records_history_main",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaRecordCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaRecordTypeHistoryMain)
				},
			},
			{
				key: "records_history_delta",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaSchemaRecordCountDataPoint(now, val, row["schema"], metadata.AttributeSchemaRecordTypeHistoryDelta)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaSchemaMemoryUsedMax.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaSchemaRecordCompressedCount.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaSchemaOperationCount.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaSchemaMemoryUsedCurrent.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaSchemaRecordCount.Enabled
		},
	},
	{
		query:                 "SELECT HOST, FREE_PHYSICAL_MEMORY, USED_PHYSICAL_MEMORY, FREE_SWAP_SPACE, USED_SWAP_SPACE, INSTANCE_TOTAL_MEMORY_USED_SIZE, INSTANCE_TOTAL_MEMORY_PEAK_USED_SIZE, INSTANCE_TOTAL_MEMORY_ALLOCATED_SIZE-INSTANCE_TOTAL_MEMORY_USED_SIZE total_free, INSTANCE_CODE_SIZE, INSTANCE_SHARED_MEMORY_ALLOCATED_SIZE, TOTAL_CPU_USER_TIME, TOTAL_CPU_SYSTEM_TIME, TOTAL_CPU_WIO_TIME, TOTAL_CPU_IDLE_TIME FROM SYS.M_HOST_RESOURCE_UTILIZATION",
		orderedResourceLabels: []string{"host"},
		orderedStats: []queryStat{
			{
				key: "free_physical_memory",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaHostMemoryCurrentDataPoint(now, val, metadata.AttributeMemoryStateUsedFreeFree)
				},
			},
			{
				key: "used_physical_memory",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaHostMemoryCurrentDataPoint(now, val, metadata.AttributeMemoryStateUsedFreeUsed)
				},
			},
			{
				key: "free_swap_space",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaHostSwapCurrentDataPoint(now, val, metadata.AttributeHostSwapStateFree)
				},
			},
			{
				key: "used_swap_space",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaHostSwapCurrentDataPoint(now, val, metadata.AttributeHostSwapStateUsed)
				},
			},
			{
				key: "instance_total_used",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaInstanceMemoryCurrentDataPoint(now, val, metadata.AttributeMemoryStateUsedFreeUsed)
				},
			},
			{
				key: "instance_total_used_peak",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaInstanceMemoryUsedPeakDataPoint(now, val)
				},
			},
			{
				key: "instance_total_free",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaInstanceMemoryCurrentDataPoint(now, val, metadata.AttributeMemoryStateUsedFreeFree)
				},
			},
			{
				key: "instance_code_size",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaInstanceCodeSizeDataPoint(now, val)
				},
			},
			{
				key: "instance_shared_memory_allocated",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaInstanceMemorySharedAllocatedDataPoint(now, val)
				},
			},
			{
				key: "cpu_user",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaCPUUsedDataPoint(now, val, metadata.AttributeCPUTypeUser)
				},
			},
			{
				key: "cpu_system",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaCPUUsedDataPoint(now, val, metadata.AttributeCPUTypeSystem)
				},
			},
			{
				key: "cpu_io_wait",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaCPUUsedDataPoint(now, val, metadata.AttributeCPUTypeIoWait)
				},
			},
			{
				key: "cpu_idle",
				addMetricFunction: func(mb *metadata.MetricsBuilder, now pcommon.Timestamp, val string,
					row map[string]string) error {
					return mb.RecordSaphanaCPUUsedDataPoint(now, val, metadata.AttributeCPUTypeIdle)
				},
			},
		},
		Enabled: func(c *Config) bool {
			return c.MetricsBuilderConfig.Metrics.SaphanaHostMemoryCurrent.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaHostSwapCurrent.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaInstanceMemoryCurrent.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaInstanceMemoryUsedPeak.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaInstanceCodeSize.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaInstanceMemorySharedAllocated.Enabled ||
				c.MetricsBuilderConfig.Metrics.SaphanaCPUUsed.Enabled
		},
	},
}

func (m *monitoringQuery) CollectMetrics(ctx context.Context, s *sapHanaScraper, client client, now pcommon.Timestamp,
	errs *scrapererror.ScrapeErrors) {
	rows, err := client.collectDataFromQuery(ctx, m)
	if err != nil {
		errs.AddPartial(len(m.orderedStats), fmt.Errorf("error running query '%s': %w", m.query, err))
		return
	}
	for _, data := range rows {
		for _, stat := range m.orderedStats {
			if err := stat.collectStat(s, m, now, data); err != nil {
				errs.AddPartial(1, err)
			}
		}
	}
}
