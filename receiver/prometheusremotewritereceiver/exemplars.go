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
//   - a hash of the remaining data labels
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
) map[exemplarKey]pmetric.ExemplarSlice {
	result := make(map[exemplarKey]pmetric.ExemplarSlice)
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

		key := makeExemplarKey(ls)
		slice, ok := result[key]
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

		result[key] = slice
	}

	return result
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

// exemplarKey identifies the series an exemplar belongs to. Prometheus defines a series by its
// labels, so the key is the whole label set: the metric name, the target and the scope labels are
// all in there, and none of them can be left out by accident.
type exemplarKey string

// makeExemplarKey builds the key for a series. labels.Bytes is documented as an opaque encoding
// usable as a map key, and the map it keys does not outlive the request that built it.
func makeExemplarKey(ls labels.Labels) exemplarKey {
	return exemplarKey(ls.Bytes(nil))
}

// sep is a byte that is not valid UTF-8, used as a field separator to prevent
// hash collisions between different field boundary combinations (e.g. "ab"+"c" vs "a"+"bc").
var sep = []byte{0xff}
