// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

// RecordPostgresqlDbSizeDataPointWithoutDatabase adds a data point to postgresql.db_size metric without a database metric attribute
func (mb *MetricsBuilder) RecordPostgresqlDbSizeDataPointWithoutDatabase(ts pcommon.Timestamp, val int64) {
	mb.metricPostgresqlDbSize.recordDatapointWithoutDatabase(mb.startTime, ts, val)
}

func (m *metricPostgresqlDbSize) recordDatapointWithoutDatabase(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordPostgresqlBackendsDataPointWithoutDatabase adds a data point to postgresql.backends metric.
func (mb *MetricsBuilder) RecordPostgresqlBackendsDataPointWithoutDatabase(ts pcommon.Timestamp, val int64) {
	mb.metricPostgresqlBackends.recordDatapointWithoutDatabase(mb.startTime, ts, val)
}

func (m *metricPostgresqlBackends) recordDatapointWithoutDatabase(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordPostgresqlBlocksReadDataPointWithoutDatabaseAndTable adds a data point to postgresql.blocks_read metric.
func (mb *MetricsBuilder) RecordPostgresqlBlocksReadDataPointWithoutDatabaseAndTable(ts pcommon.Timestamp, val int64, sourceAttributeValue AttributeSource) {
	mb.metricPostgresqlBlocksRead.recordDatapointWithoutDatabaseAndTable(mb.startTime, ts, val, sourceAttributeValue.String())
}

func (m *metricPostgresqlBlocksRead) recordDatapointWithoutDatabaseAndTable(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, sourceAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("source", sourceAttributeValue)
}

// RecordPostgresqlCommitsDataPointWithoutDatabase adds a data point to postgresql.commits metric without the database metric attribute
func (mb *MetricsBuilder) RecordPostgresqlCommitsDataPointWithoutDatabase(ts pcommon.Timestamp, val int64) {
	mb.metricPostgresqlCommits.recordDatapointWithoutDatabase(mb.startTime, ts, val)
}

func (m *metricPostgresqlCommits) recordDatapointWithoutDatabase(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordPostgresqlRollbacksDataPointWithoutDatabase adds a data point to postgresql.commits metric without the database metric attribute
func (mb *MetricsBuilder) RecordPostgresqlRollbacksDataPointWithoutDatabase(ts pcommon.Timestamp, val int64) {
	mb.metricPostgresqlRollbacks.recordDatapointWithoutDatabase(mb.startTime, ts, val)
}

func (m *metricPostgresqlRollbacks) recordDatapointWithoutDatabase(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordPostgresqlRowsDataPointWithoutDatabaseAndTable adds a data point to postgresql.rows metric without the database or table metric attribute.
func (mb *MetricsBuilder) RecordPostgresqlRowsDataPointWithoutDatabaseAndTable(ts pcommon.Timestamp, val int64, stateAttributeValue AttributeState) {
	mb.metricPostgresqlRows.recordDatapointWithoutDatabaseAndTable(mb.startTime, ts, val, stateAttributeValue.String())
}

func (m *metricPostgresqlRows) recordDatapointWithoutDatabaseAndTable(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, stateAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("state", stateAttributeValue)
}

// RecordPostgresqlOperationsDataPointWithoutDatabaseAndTable adds a data point to postgresql.operations metric without the database or table metric attribute
func (mb *MetricsBuilder) RecordPostgresqlOperationsDataPointWithoutDatabaseAndTable(ts pcommon.Timestamp, val int64, operationAttributeValue AttributeOperation) {
	mb.metricPostgresqlOperations.recordDatapointWithoutDatabaseAndTable(mb.startTime, ts, val, operationAttributeValue.String())
}

func (m *metricPostgresqlOperations) recordDatapointWithoutDatabaseAndTable(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, operationAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("operation", operationAttributeValue)
}
