// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"encoding/hex"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/schema"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// collectExemplars extracts Prometheus exemplars from a writev2 request and
// groups them into ExemplarSlices keyed by metric identity.
//
// Exemplars are grouped by a hash composed of:
//   - instrumentation scope name
//   - instrumentation scope version
//   - metric name
//   - metric type
//
// TODO:
//
//	Right now, remote-write 2.0 sends disconnected exemplars without histogram, which requires
//	caching exemplars and associating them later with histogram data points.
//	Once https://github.com/prometheus/prometheus/issues/17857 is resolved, we can optimize this
func collectExemplars(
	req *writev2.Request,
	settings receiver.Settings,
	stats *promremote.WriteResponseStats,
) map[uint64]pmetric.ExemplarSlice {
	result := make(map[uint64]pmetric.ExemplarSlice)
	builder := labels.NewScratchBuilder(0)
	stats.Exemplars = 0
	for i := range req.Timeseries {
		ts := &req.Timeseries[i]
		if len(ts.Exemplars) == 0 {
			continue
		}

		ls, err := ts.ToLabels(&builder, req.Symbols)
		if err != nil {
			settings.Logger.Warn("failed to extract labels from request symbols", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
			continue
		}

		metadata := schema.NewMetadataFromLabels(ls)
		if metadata.Name == "" {
			settings.Logger.Warn("missing metric name in labels")
			continue
		}

		scopeName, scopeVersion := extractScopeFromLabels(settings, ls)

		key := exemplarKey{
			ScopeName:    scopeName,
			ScopeVersion: scopeVersion,
			MetricName:   metadata.Name,
			MetricType:   ts.Metadata.Type,
			AttrsHash:    pdatautil.MapHash(extractAttributes(ls)),
		}

		slice, ok := result[key.hash()]
		if !ok {
			slice = pmetric.NewExemplarSlice()
		}

		for _, ex := range ts.Exemplars {
			promExemplar, err := ex.ToExemplar(&builder, req.Symbols)
			if err != nil {
				settings.Logger.Warn("error converting exemplar label refs", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
				continue
			}

			exemplar := slice.AppendEmpty()
			exemplar.SetTimestamp(pcommon.Timestamp(ex.Timestamp * int64(time.Millisecond)))
			exemplar.SetDoubleValue(ex.Value)

			setTraceAndSpan(exemplar, promExemplar.Labels)
			copyExemplarAttributes(exemplar.FilteredAttributes(), promExemplar.Labels)
			stats.Exemplars++
		}

		result[key.hash()] = slice
	}

	return result
}

func extractScopeFromLabels(settings receiver.Settings, ls labels.Labels) (string, string) {
	name := settings.BuildInfo.Description
	version := settings.BuildInfo.Version

	if sName := ls.Get("otel_scope_name"); sName != "" {
		name = sName
	}
	if sVersion := ls.Get("otel_scope_version"); sVersion != "" {
		version = sVersion
	}
	return name, version
}

// decodeTraceID decodes a trace ID string and returns a valid pcommon.TraceID
// if the string is exactly 32 hex characters and not an all-zero ID.
func decodeTraceID(tid string) (pcommon.TraceID, bool) {
	if len(tid) != 32 {
		return pcommon.TraceID{}, false
	}
	var t [16]byte
	if b, err := hex.DecodeString(tid); err == nil {
		copy(t[:], b)
		id := pcommon.TraceID(t)
		if !id.IsEmpty() {
			return id, true
		}
	}
	return pcommon.TraceID{}, false
}

// decodeSpanID decodes a span ID string and returns a valid pcommon.SpanID
// if the string is exactly 16 hex characters and not an all-zero ID.
func decodeSpanID(sid string) (pcommon.SpanID, bool) {
	if len(sid) != 16 {
		return pcommon.SpanID{}, false
	}
	var s [8]byte
	if b, err := hex.DecodeString(sid); err == nil {
		copy(s[:], b)
		id := pcommon.SpanID(s)
		if !id.IsEmpty() {
			return id, true
		}
	}
	return pcommon.SpanID{}, false
}

// setTraceAndSpan extracts trace ID and span ID from exemplar labels
// and sets them on the provided Exemplar if they are valid non-zero hex-encoded IDs.
func setTraceAndSpan(exemplar pmetric.Exemplar, labels labels.Labels) {
	if id, ok := decodeTraceID(labels.Get(prometheus.ExemplarTraceIDKey)); ok {
		exemplar.SetTraceID(id)
	}
	if id, ok := decodeSpanID(labels.Get(prometheus.ExemplarSpanIDKey)); ok {
		exemplar.SetSpanID(id)
	}
}

// copyExemplarAttributes copies labels into the destination attribute map.
// Valid trace_id and span_id labels that were converted to the exemplar's
// TraceID and SpanID are omitted, while invalid or other labels are preserved.
func copyExemplarAttributes(dest pcommon.Map, labels labels.Labels) {
	for k, v := range labels.Map() {
		if k == prometheus.ExemplarTraceIDKey {
			if _, ok := decodeTraceID(v); ok {
				continue
			}
		} else if k == prometheus.ExemplarSpanIDKey {
			if _, ok := decodeSpanID(v); ok {
				continue
			}
		}
		dest.PutStr(k, v)
	}
}

type exemplarKey struct {
	ScopeName    string
	ScopeVersion string
	MetricName   string
	MetricType   writev2.Metadata_MetricType
	AttrsHash    [16]byte // hash of data labels (excludes job, instance, __name__, otel_scope_*)
}

// sep is a byte that is not valid UTF-8, used as a field separator to prevent
// hash collisions between different field boundary combinations (e.g. "ab"+"c" vs "a"+"bc").
var sep = []byte{0xff}

func (k exemplarKey) hash() uint64 {
	h := identity.Resource{}.Hash()
	h.Write([]byte(k.ScopeName))
	h.Write(sep)
	h.Write([]byte(k.ScopeVersion))
	h.Write(sep)
	h.Write([]byte(k.MetricName))
	h.Write(sep)
	h.Write([]byte(k.MetricType.String()))
	h.Write(sep)
	h.Write(k.AttrsHash[:])
	return h.Sum64()
}
