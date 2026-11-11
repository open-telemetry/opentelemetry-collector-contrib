// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

const (
	// language=SQL
	createDatabaseSQL = `CREATE KEYSPACE IF NOT EXISTS %s WITH REPLICATION = { 'class' : '%s', 'replication_factor' : %d };`
	// language=SQL
	// span_event is a new type name. The previous events type stored a single date and cannot be altered in place.
	createEventTypeSQL = `CREATE TYPE IF NOT EXISTS %s.span_event (ts timestamp, name text, attributes map<text, text>);`
	// language=SQL
	createLinksTypeSQL = `CREATE TYPE IF NOT EXISTS %s.span_link (trace_id text, span_id text, trace_state text, attributes map<text, text>);`
	// language=SQL
	createSpanTableSQL = `CREATE TABLE IF NOT EXISTS %s.%s (ts timestamp, trace_id text, span_id text, parent_span_id text, trace_state text, span_name text, span_kind text, resource_attributes map<text, text>, span_attributes map<text, text>, duration bigint, status_code text, status_message text, events list<frozen<span_event>>, links list<frozen<span_link>>, PRIMARY KEY ((trace_id), span_id)) WITH COMPRESSION = {'class': '%s'}`
	// language=SQL
	insertSpanSQL = `INSERT INTO %s.%s (ts, trace_id, span_id, parent_span_id, trace_state, span_name, span_kind, resource_attributes, span_attributes, duration, status_code, status_message, events, links) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// language=SQL
	createLogTableSQL = `CREATE TABLE IF NOT EXISTS %s.%s (id uuid, ts timestamp, partition_key text, trace_id text, span_id text, trace_flags int, severity_text text, severity_number int, body text, resource_attributes map<text, text>, log_attributes map<text, text>, PRIMARY KEY ((partition_key), ts, id)) WITH COMPRESSION = {'class': '%s'}`
	// language=SQL
	insertLogTableSQL = `INSERT INTO %s.%s (id, ts, partition_key, trace_id, span_id, trace_flags, severity_text, severity_number, body, resource_attributes, log_attributes) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

// maxStatementsPerBatch bounds an unlogged batch to one partition.
// Cassandra rejects batches that exceed batch_size_fail_threshold_in_kb (default 50KiB).
const maxStatementsPerBatch = 32
