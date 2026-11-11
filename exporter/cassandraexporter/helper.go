// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"crypto/sha256"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

func attributesToMap(attributes map[string]any) map[string]string {
	newAttrMap := make(map[string]string)
	for k, v := range attributes {
		jsonV, err := json.Marshal(v)
		if err == nil {
			newAttrMap[k] = string(jsonV)
		}
	}
	return newAttrMap
}

// logPartitionKey keeps logs for one trace in a single partition.
// Logs with no trace id are spread across hourly buckets so they do not share one hot partition.
func logPartitionKey(traceID string, ts time.Time) string {
	if traceID != "" {
		return traceID
	}
	return "untraced/" + ts.UTC().Format("2006-01-02T15")
}

// logRecordID is deterministic for a record's position in an export request.
// A retry of the same request overwrites the previous row instead of inserting a duplicate.
func logRecordID(index int, partitionKey, traceID, spanID string, ts time.Time, flags uint32, severityText string, severity int32, body string, resourceAttrs, logAttrs map[string]string) gocql.UUID {
	h := sha256.New()
	add := func(s string) {
		_, _ = io.WriteString(h, s)
		_, _ = h.Write([]byte{0})
	}
	add(strconv.Itoa(index))
	add(partitionKey)
	add(traceID)
	add(spanID)
	add(strconv.FormatInt(ts.UnixNano(), 10))
	add(strconv.FormatUint(uint64(flags), 10))
	add(severityText)
	add(strconv.FormatInt(int64(severity), 10))
	add(body)
	add(canonicalMap(resourceAttrs))
	add(canonicalMap(logAttrs))

	sum := h.Sum(nil)
	var id gocql.UUID
	copy(id[:], sum[:16])
	id[6] = (id[6] & 0x0f) | 0x50
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

func canonicalMap(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(values[key])
		b.WriteByte(0)
	}
	return b.String()
}

type spanEvent struct {
	Timestamp  time.Time         `cql:"ts"`
	Name       string            `cql:"name"`
	Attributes map[string]string `cql:"attributes"`
}

type spanLink struct {
	TraceID    string            `cql:"trace_id"`
	SpanID     string            `cql:"span_id"`
	TraceState string            `cql:"trace_state"`
	Attributes map[string]string `cql:"attributes"`
}

func eventsFromSpan(span ptrace.Span) []spanEvent {
	events := make([]spanEvent, 0, span.Events().Len())
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		events = append(events, spanEvent{
			Timestamp:  event.Timestamp().AsTime(),
			Name:       event.Name(),
			Attributes: attributesToMap(event.Attributes().AsRaw()),
		})
	}
	return events
}

func linksFromSpan(span ptrace.Span) []spanLink {
	links := make([]spanLink, 0, span.Links().Len())
	for i := 0; i < span.Links().Len(); i++ {
		link := span.Links().At(i)
		links = append(links, spanLink{
			TraceID:    traceutil.TraceIDToHexOrEmptyString(link.TraceID()),
			SpanID:     traceutil.SpanIDToHexOrEmptyString(link.SpanID()),
			TraceState: link.TraceState().AsRaw(),
			Attributes: attributesToMap(link.Attributes().AsRaw()),
		})
	}
	return links
}
