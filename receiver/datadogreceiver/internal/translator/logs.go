// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"maps"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// DatadogLogPayload mirrors an item of Datadog's logs intake (POST /api/v2/logs). The Datadog Agent
// sends an array of these as gzip-compressed JSON. Datadog's HTTPLogItem only defines a handful of
// reserved fields (message, status, hostname, service, ddsource, ddtags); any other JSON properties
// are arbitrary structured attributes that we must preserve rather than drop. Those land in
// Additional via the custom UnmarshalJSON below.
//
// See https://docs.datadoghq.com/api/latest/logs/#send-logs and
// https://github.com/DataDog/datadog-api-client-go/blob/master/api/datadogV2/model_http_log_item.go
type DatadogLogPayload struct {
	Message  string
	Status   string
	Hostname string
	Service  string
	Source   string // ddsource
	Tags     string // ddtags
	// Timestamp is the Datadog log timestamp in Unix epoch milliseconds, when supplied as a number.
	// Datadog also accepts ISO8601/RFC3339 strings and alternate keys (@timestamp, date); those are
	// resolved from Additional by resolveTimestamp.
	Timestamp int64
	// Additional holds every non-reserved JSON property. Numbers are decoded as json.Number so that
	// 64-bit identifiers (e.g. dd.trace_id) survive without float precision loss.
	Additional map[string]any
}

// ddTimestampKeys are the attribute keys Datadog may use to carry a log's timestamp, in priority
// order. "timestamp" is handled via the typed field first; the rest are checked in Additional.
var ddTimestampKeys = []string{"timestamp", "@timestamp", "date", "_timestamp"}

// handledAdditionalKeys are Additional keys promoted into dedicated OTel slots (resource attributes,
// trace context, or the timestamp). They must not be re-emitted as raw log attributes.
var handledAdditionalKeys = map[string]struct{}{
	"dd.trace_id": {},
	"dd.span_id":  {},
	"_dd.p.tid":   {},
	"dd.service":  {},
	"dd.env":      {},
	"dd.version":  {},
	"@timestamp":  {},
	"date":        {},
	"_timestamp":  {},
	"timestamp":   {},
}

func (p *DatadogLogPayload) UnmarshalJSON(data []byte) error {
	// Decode in a single pass with UseNumber so 64-bit identifiers (e.g. dd.trace_id) keep full
	// precision; reserved keys populate the typed fields and the rest land in Additional.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	all := map[string]any{}
	if err := dec.Decode(&all); err != nil {
		return err
	}

	p.Additional = make(map[string]any, len(all))
	for k, v := range all {
		switch k {
		case "message":
			p.Message, _ = v.(string)
		case "status":
			p.Status, _ = v.(string)
		case "hostname":
			p.Hostname, _ = v.(string)
		case "service":
			p.Service, _ = v.(string)
		case "ddsource":
			p.Source, _ = v.(string)
		case "ddtags":
			p.Tags, _ = v.(string)
		case "timestamp":
			// Numeric epoch-ms uses the typed field; a non-numeric form (e.g. ISO8601 string) stays
			// in Additional for resolveTimestamp to handle.
			if n, ok := v.(json.Number); ok {
				if ms, err := n.Int64(); err == nil {
					p.Timestamp = ms

					continue
				}
			}

			p.Additional[k] = v
		default:
			p.Additional[k] = v
		}
	}
	return nil
}

// ToPlog translates a batch of Datadog log items into OTLP logs. receivedAt is the time the request
// was received and is used for ObservedTimestamp. When decodeJSONMessage is true, records whose
// message is itself a JSON object are expanded (see decodeJSONMessagePayload). Records that resolve to
// the same resource attributes are grouped under a single ResourceLogs.
func ToPlog(incomingLogs []*DatadogLogPayload, receivedAt time.Time, decodeJSONMessage bool) plog.Logs {
	logs := plog.NewLogs()
	if len(incomingLogs) == 0 {
		return logs
	}

	observed := pcommon.NewTimestampFromTime(receivedAt)
	pool := newStringPool()
	// Group records sharing identical resource attributes into one ResourceLogs.
	scopeByResource := make(map[string]plog.ScopeLogs)

	for _, in := range incomingLogs {
		if in == nil {
			continue
		}
		if decodeJSONMessage {
			in = decodeJSONMessagePayload(in)
		}

		// Resolve resource and log-level attributes. ddtags reuses the metrics tag parser so logs get
		// the same key:value parsing and known-key to resource-attribute promotion (env, version,
		// container, cloud, k8s, etc.). hostname maps to host.name inside tagsToAttributes.
		var tags []string
		if in.Tags != "" {
			tags = strings.Split(in.Tags, ",")
		}

		attrs := tagsToAttributes(tags, in.Hostname, pool)
		if in.Service != "" {
			attrs.resource.PutStr(string(conventions.ServiceNameKey), in.Service)
		}

		applyReservedDDResourceAttributes(attrs.resource, in.Additional)

		sl, ok := scopeByResource[resourceKey(attrs.resource)]
		if !ok {
			rl := logs.ResourceLogs().AppendEmpty()
			attrs.resource.CopyTo(rl.Resource().Attributes())
			sl = rl.ScopeLogs().AppendEmpty()
			scopeByResource[resourceKey(attrs.resource)] = sl
		}

		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr(in.Message)
		lr.SetObservedTimestamp(observed)
		if ts, ok := in.resolveTimestamp(); ok {
			lr.SetTimestamp(ts)
		}

		if in.Status != "" {
			lr.SetSeverityText(in.Status)
			lr.SetSeverityNumber(statusToSeverityNumber(in.Status))
		}

		if in.Source != "" {
			lr.Attributes().PutStr("datadog.ddsource", in.Source)
		}

		setTraceContext(lr, in.Additional)

		// Log-level attributes parsed out of ddtags.
		attrs.dp.Range(func(k string, v pcommon.Value) bool {
			v.CopyTo(lr.Attributes().PutEmpty(k))
			return true
		})

		// Arbitrary structured attributes from the payload (excluding those already promoted above).
		addAdditionalAttributes(lr.Attributes(), in.Additional)
	}

	return logs
}

// decodeJSONMessagePayload mirrors Datadog's server-side "Preprocessing for JSON logs". When the
// agent forwards an application JSON log, the whole line arrives as an opaque message string. If that
// string is a JSON object, we parse it and let its reserved attributes (message, status/level,
// timestamp, host, service, ddsource, and dd.trace_id/dd.span_id for correlation) take precedence over
// the agent envelope. Remaining inner keys become attributes. Non-JSON messages are returned
// unchanged. The returned payload is a copy, the input is never mutated.
func decodeJSONMessagePayload(in *DatadogLogPayload) *DatadogLogPayload {
	if !strings.HasPrefix(strings.TrimSpace(in.Message), "{") {
		return in
	}

	dec := json.NewDecoder(strings.NewReader(in.Message))
	dec.UseNumber()

	var inner map[string]any
	if err := dec.Decode(&inner); err != nil {
		return in // not valid JSON; leave the message as-is
	}

	out := *in
	out.Additional = make(map[string]any, len(in.Additional)+len(inner))
	maps.Copy(out.Additional, in.Additional)

	if s, ok := stringAttribute(inner["message"]); ok {
		out.Message = s
	}

	if s, ok := firstStringAttribute(inner, "status", "level", "severity"); ok {
		out.Status = s
	}

	if s, ok := firstStringAttribute(inner, "hostname", "host"); ok {
		out.Hostname = s
	}

	if s, ok := stringAttribute(inner["service"]); ok {
		out.Service = s
	}

	if s, ok := stringAttribute(inner["ddsource"]); ok {
		out.Source = s
	}

	// If the inner log carries its own timestamp, it reflects the application's emit time and should
	// win over the agent's collection time. Clear the typed field and let resolveTimestamp parse the
	// inner value (numeric epoch-ms or ISO8601) from Additional.
	for _, k := range ddTimestampKeys {
		if v, ok := inner[k]; ok {
			out.Timestamp = 0
			out.Additional[k] = v

			break
		}
	}

	// Remaining (non-reserved) inner keys become attributes; the inner values win over the envelope.
	for k, v := range inner {
		switch strings.ToLower(k) {
		case "message", "status", "level", "severity", "hostname", "host", "service", "ddsource", "ddtags", "timestamp", "@timestamp", "date", "_timestamp":
			continue
		}

		out.Additional[k] = v
	}

	return &out
}

func firstStringAttribute(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if s, ok := stringAttribute(m[k]); ok {
			return s, true
		}
	}

	return "", false
}

// resolveTimestamp returns the log's timestamp as an OTel timestamp. Datadog log timestamps are Unix
// epoch milliseconds when numeric; ISO8601/RFC3339 strings are also accepted. Returns false when
// no timestamp is present (callers should rely on ObservedTimestamp).
func (p *DatadogLogPayload) resolveTimestamp() (pcommon.Timestamp, bool) {
	if p.Timestamp != 0 {
		return millisToTimestamp(p.Timestamp), true
	}

	for _, k := range ddTimestampKeys {
		v, ok := p.Additional[k]
		if !ok {
			continue
		}

		if ts, ok := parseTimestampValue(v); ok {
			return ts, true
		}
	}

	return 0, false
}

// millisToTimestamp converts Unix epoch milliseconds to an OTel nanosecond timestamp.
func millisToTimestamp(ms int64) pcommon.Timestamp {
	return pcommon.Timestamp(ms * int64(time.Millisecond))
}

func parseTimestampValue(v any) (pcommon.Timestamp, bool) {
	switch val := v.(type) {
	case json.Number:
		if ms, err := val.Int64(); err == nil && ms != 0 {
			return millisToTimestamp(ms), true
		}
	case float64:
		if val != 0 {
			return millisToTimestamp(int64(val)), true
		}
	case string:
		if val == "" {
			return 0, false
		}
		if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms != 0 {
			return millisToTimestamp(ms), true
		}
		if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
			return pcommon.NewTimestampFromTime(t), true
		}
	}
	return 0, false
}

// statusToSeverityNumber maps Datadog log statuses (a syslog-derived set) to OTel SeverityNumbers
// using the canonical mapping from the OpenTelemetry logs data model appendix.
func statusToSeverityNumber(status string) plog.SeverityNumber {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "trace":
		return plog.SeverityNumberTrace
	case "debug":
		return plog.SeverityNumberDebug
	case "info", "informational", "ok":
		return plog.SeverityNumberInfo
	case "notice":
		return plog.SeverityNumberInfo2
	case "warn", "warning":
		return plog.SeverityNumberWarn
	case "error", "err":
		return plog.SeverityNumberError
	case "critical", "crit":
		return plog.SeverityNumberError2
	case "alert":
		return plog.SeverityNumberError3
	case "emergency", "emerg", "fatal":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}

// setTraceContext promotes Datadog log-injection correlation fields onto the LogRecord so logs can be
// joined to traces. Datadog renders dd.trace_id as a decimal integer (64- or 128-bit) or, for
// OTel-origin traces, 32-char hex; the upper 64 bits of a 128-bit id may instead arrive in _dd.p.tid
// (hex), exactly as the trace translator handles for spans. parseDatadogTraceID reconstructs the full
// 128-bit id so logs and spans produce identical TraceIDs.
func setTraceContext(lr plog.LogRecord, additional map[string]any) {
	tidHex, _ := stringAttribute(additional["_dd.p.tid"])

	if traceID, ok := parseDatadogTraceID(additional["dd.trace_id"], tidHex); ok {
		lr.SetTraceID(traceID)
	}

	if spanID, ok := parseDatadogSpanID(additional["dd.span_id"]); ok {
		lr.SetSpanID(spanID)
	}
}

// parseDatadogTraceID parses a Datadog dd.trace_id into a 128-bit OTel TraceID. A 64-bit value
// occupies the low 64 bits (zero-padded high), matching the span translator; when only 64 bits are
// present and upperHex (_dd.p.tid) is supplied, it fills the high 64 bits to reconstruct the full id.
func parseDatadogTraceID(v any, upperHex string) (pcommon.TraceID, bool) {
	s, ok := stringAttribute(v)
	if !ok || s == "" {
		return pcommon.TraceID{}, false
	}

	var id pcommon.TraceID
	// OTel-origin traces render the id as hex (32 chars for 128-bit, 16 for 64-bit). Native Datadog
	// renders it as a decimal integer; a hex letter disambiguates hex from decimal.
	if containsHexLetter(s) {
		if b, err := hex.DecodeString(s); err == nil && len(b) <= len(id) {
			copy(id[len(id)-len(b):], b)

			return id, !id.IsEmpty()
		}

		return pcommon.TraceID{}, false
	}

	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.BitLen() > 128 {
		return pcommon.TraceID{}, false
	}

	n.FillBytes(id[:])
	if upperHex != "" && binary.BigEndian.Uint64(id[:8]) == 0 {
		if upper, err := strconv.ParseUint(upperHex, 16, 64); err == nil {
			binary.BigEndian.PutUint64(id[:8], upper)
		}
	}

	return id, !id.IsEmpty()
}

// parseDatadogSpanID parses a Datadog dd.span_id (decimal, or 16-char hex for OTel-origin spans) into
// a 64-bit OTel SpanID.
func parseDatadogSpanID(v any) (pcommon.SpanID, bool) {
	s, ok := stringAttribute(v)

	if !ok || s == "" {
		return pcommon.SpanID{}, false
	}

	var sp pcommon.SpanID
	if containsHexLetter(s) {
		if b, err := hex.DecodeString(s); err == nil && len(b) <= len(sp) {
			copy(sp[len(sp)-len(b):], b)

			return sp, !sp.IsEmpty()
		}

		return pcommon.SpanID{}, false
	}

	if id, err := strconv.ParseUint(s, 10, 64); err == nil {
		return uInt64ToSpanID(id), id != 0
	}

	return pcommon.SpanID{}, false
}

func containsHexLetter(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			return true
		}
	}

	return false
}

// applyReservedDDResourceAttributes maps Datadog log-injection service identity fields to OTel
// resource attributes. These take precedence over values derived from ddtags.
func applyReservedDDResourceAttributes(resource pcommon.Map, additional map[string]any) {
	if v, ok := stringAttribute(additional["dd.service"]); ok {
		resource.PutStr(string(conventions.ServiceNameKey), v)
	}

	if v, ok := stringAttribute(additional["dd.env"]); ok {
		resource.PutStr(string(conventions.DeploymentEnvironmentNameKey), v)
	}

	if v, ok := stringAttribute(additional["dd.version"]); ok {
		resource.PutStr(string(conventions.ServiceVersionKey), v)
	}
}

// addAdditionalAttributes copies arbitrary payload properties onto the log record's attributes,
// translating known Datadog keys to OTel semantic conventions and skipping keys already promoted to
// dedicated slots.
func addAdditionalAttributes(attrs pcommon.Map, additional map[string]any) {
	keys := make([]string, 0, len(additional))
	for k := range additional {
		keys = append(keys, k)
	}

	sort.Strings(keys) // deterministic ordering for stable test output
	for _, k := range keys {
		if _, handled := handledAdditionalKeys[k]; handled {
			continue
		}

		putAnyValue(attrs, translateDatadogKeyToOTel(k), additional[k])
	}
}

// stringAttribute extracts a string from a decoded JSON value (string or json.Number).
func stringAttribute(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		if val == "" {
			return "", false
		}

		return val, true
	case json.Number:
		return val.String(), true
	}

	return "", false
}

// putAnyValue inserts a decoded JSON value into a pcommon.Map with the appropriate OTel value type.
func putAnyValue(attrs pcommon.Map, key string, v any) {
	switch val := v.(type) {
	case string:
		attrs.PutStr(key, val)
	case bool:
		attrs.PutBool(key, val)
	case json.Number:
		if i, err := val.Int64(); err == nil {
			attrs.PutInt(key, i)
		} else if f, err := val.Float64(); err == nil {
			attrs.PutDouble(key, f)
		} else {
			attrs.PutStr(key, val.String())
		}
	case float64:
		attrs.PutDouble(key, val)
	case map[string]any:
		dest := attrs.PutEmptyMap(key)
		nested := make([]string, 0, len(val))
		for k := range val {
			nested = append(nested, k)
		}

		sort.Strings(nested)
		for _, k := range nested {
			putAnyValue(dest, k, val[k])
		}
	case []any:
		dest := attrs.PutEmptySlice(key)
		for _, item := range val {
			appendAnyValue(dest, item)
		}
	case nil:
		attrs.PutEmpty(key)
	default:
		attrs.PutStr(key, "")
	}
}

func appendAnyValue(slice pcommon.Slice, v any) {
	switch val := v.(type) {
	case string:
		slice.AppendEmpty().SetStr(val)
	case bool:
		slice.AppendEmpty().SetBool(val)
	case json.Number:
		if i, err := val.Int64(); err == nil {
			slice.AppendEmpty().SetInt(i)
		} else if f, err := val.Float64(); err == nil {
			slice.AppendEmpty().SetDouble(f)
		} else {
			slice.AppendEmpty().SetStr(val.String())
		}
	case float64:
		slice.AppendEmpty().SetDouble(val)
	default:
		slice.AppendEmpty()
	}
}

// resourceKey builds a deterministic key from a resource attribute map so logs with identical
// resources are grouped together.
func resourceKey(m pcommon.Map) string {
	keys := make([]string, 0, m.Len())
	m.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})

	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		v, _ := m.Get(k)
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(v.AsString())
		sb.WriteByte(0)
	}

	return sb.String()
}
