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

			traceIDSet, spanIDSet := setTraceAndSpan(exemplar, promExemplar.Labels)
			copyExemplarAttributes(exemplar.FilteredAttributes(), promExemplar.Labels, traceIDSet, spanIDSet)
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

// setTraceAndSpan converts the hex-encoded trace and span ID labels and reports
// which ones it consumed, so the caller can keep a rejected value as a filtered
// attribute.
//
// Only valid IDs are converted: the label must decode to exactly the
// OpenTelemetry ID width and must not be all zero. Anything else is left for the
// caller rather than zero padded or truncated.
func setTraceAndSpan(exemplar pmetric.Exemplar, labels labels.Labels) (traceIDSet, spanIDSet bool) {
	if tid := labels.Get(prometheus.ExemplarTraceIDKey); tid != "" {
		var t [16]byte
		if len(tid) == hex.EncodedLen(len(t)) {
			if b, err := hex.DecodeString(tid); err == nil {
				copy(t[:], b)
				// all-zero is the "unset" sentinel: setting it would drop the label for nothing
				if traceID := pcommon.TraceID(t); !traceID.IsEmpty() {
					exemplar.SetTraceID(traceID)
					traceIDSet = true
				}
			}
		}
	}
	if sid := labels.Get(prometheus.ExemplarSpanIDKey); sid != "" {
		var s [8]byte
		if len(sid) == hex.EncodedLen(len(s)) {
			if b, err := hex.DecodeString(sid); err == nil {
				copy(s[:], b)
				if spanID := pcommon.SpanID(s); !spanID.IsEmpty() {
					exemplar.SetSpanID(spanID)
					spanIDSet = true
				}
			}
		}
	}
	return traceIDSet, spanIDSet
}

// copyExemplarAttributes copies all labels into dest, skipping the trace and span
// ID labels only when setTraceAndSpan converted them. A rejected label is copied
// like any other, so the value the sender wrote is not lost.
func copyExemplarAttributes(dest pcommon.Map, labels labels.Labels, traceIDSet, spanIDSet bool) {
	for k, v := range labels.Map() {
		if k == prometheus.ExemplarTraceIDKey && traceIDSet {
			continue
		}
		if k == prometheus.ExemplarSpanIDKey && spanIDSet {
			continue
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
