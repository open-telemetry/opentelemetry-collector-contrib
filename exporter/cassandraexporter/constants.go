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

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

const (
	// language=SQL
	createDatabaseSQL = `CREATE KEYSPACE %s WITH REPLICATION = { 'class' : '%s', 'replication_factor' : %d };`
	// language=SQL
	createEventTypeSQL = `CREATE TYPE IF NOT EXISTS %s.Events (Timestamp Date, Name text, Attributes map<text, text>);`
	// language=SQL
	createLinksTypeSQL = `CREATE TYPE IF NOT EXISTS %s.Links (TraceId text, SpanId text, TraceState text, Attributes map<text, text>);`
	// language=SQL
	createSpanTableSQL = `CREATE TABLE IF NOT EXISTS %s.%s (TimeStamp DATE, TraceId text, SpanId text, ParentSpanId text, TraceState text, SpanName text, SpanKind text, ResourceAttributes map<text, text>, SpanAttributes map<text, text>, Duration int, StatusCode text, StatusMessage text, Events frozen<Events>, Links frozen<Links>, PRIMARY KEY (SpanId)) WITH COMPRESSION = {'class': '%s'}`
	// language=SQL
	insertSpanSQL = `INSERT INTO %s.%s (timestamp, traceid, spanid, parentspanid, tracestate, spanname, spankind, resourceattributes, spanattributes, duration, statuscode, statusmessage) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// language=SQL
	createLogTableSQL = `CREATE TABLE IF NOT EXISTS %s.%s (TimeStamp TimeStamp, TraceId text, SpanId text, TraceFlags int, SeverityText text, SeverityNumber int, Body text, ResourceAttributes map<text, text>, LogAttributes map<text, text>, PRIMARY KEY (SpanId, SeverityNumber)) WITH COMPRESSION = {'class': '%s'}`
	// language=SQL
	insertLogTableSQL = `INSERT INTO %s.%s (timestamp, traceid, spanid, traceflags, severitytext, severitynumber, body, resourceattributes, logattributes) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`
)
