// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/schema"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// collectExemplars extracts Prometheus exemplars from a writev2 request and
// groups them into ExemplarSlices keyed by metric identity.
//
// Exemplars are grouped by a hash composed of:
//   - job and instance, which is what the resource is built from
//   - instrumentation scope name
//   - instrumentation scope version
//   - metric name
//   - metric type
//   - a hash of the remaining data labels
//
// The same key has to be rebuilt when the exemplars are attached, otherwise they are collected
// and then never found again.
//
// TODO:
//
//	Right now, remote-write 2.0 sends disconnected exemplars without histogram, which requires
//	caching exemplars and associating them later with histogram data points.
//	Once https://github.com/prometheus/prometheus/issues/17857 is resolved, we can optimize this
func (prw *prometheusRemoteWriteReceiver) collectExemplars(
	req *writev2.Request,
) map[uint64]pmetric.ExemplarSlice {
	settings := prw.settings
	result := make(map[uint64]pmetric.ExemplarSlice)
	builder := labels.NewScratchBuilder(0)
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

		si := prw.extractScopeInfo(ls)
		unit := ""
		if ts.Metadata.UnitRef < uint32(len(req.Symbols)) {
			unit = req.Symbols[ts.Metadata.UnitRef]
		}

		key := exemplarKey{
			Job:            ls.Get("job"),
			Instance:       ls.Get("instance"),
			ScopeName:      si.Name,
			ScopeVersion:   si.Version,
			ScopeSchemaURL: si.SchemaURL,
			ScopeAttrs:     scopeAttrsKey(si.scopeAttrs),
			MetricName:     metadata.Name,
			Unit:           unit,
			MetricType:     ts.Metadata.Type,
			AttrsHash:      pdatautil.MapHash(extractAttributes(ls)),
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
		}

		result[key.hash()] = slice
	}

	return result
}

// setTraceAndSpan extracts trace ID and span ID from exemplar labels
// and sets them on the provided Exemplar.
//
// The function expects hexadecimal-encoded IDs using Prometheus
// exemplar label keys and silently ignores invalid values.
func setTraceAndSpan(exemplar pmetric.Exemplar, labels labels.Labels) {
	// An ID of the wrong length is not the ID that was sent. Padding a short one or cutting a long
	// one down would hand the pipeline a different trace than the sender named.
	if tid := labels.Get(prometheus.ExemplarTraceIDKey); tid != "" {
		var t [16]byte
		if b, err := hex.DecodeString(tid); err == nil && len(b) == len(t) {
			exemplar.SetTraceID(pcommon.TraceID(t[:copy(t[:], b)]))
		}
	}
	if sid := labels.Get(prometheus.ExemplarSpanIDKey); sid != "" {
		var s [8]byte
		if b, err := hex.DecodeString(sid); err == nil && len(b) == len(s) {
			exemplar.SetSpanID(pcommon.SpanID(s[:copy(s[:], b)]))
		}
	}
}

// copyExemplarAttributes copies all labels into the destination attribute map
// except for trace ID and span ID labels, which are handled separately.
//
// The destination map is typically the exemplar's filtered attributes.
func copyExemplarAttributes(dest pcommon.Map, labels labels.Labels) {
	for k, v := range labels.Map() {
		if k == prometheus.ExemplarTraceIDKey || k == prometheus.ExemplarSpanIDKey {
			continue
		}
		dest.PutStr(k, v)
	}
}

// exemplarKey names the data point an exemplar belongs to. Every field the receiver uses to tell
// one output metric from another has to be here, or two of them share a single set of exemplars.
type exemplarKey struct {
	// Job and Instance identify the target. They are what the resource is built from, so leaving
	// them out lets two targets publishing the same series share one another's exemplars.
	Job      string
	Instance string
	// The scope is its name, version, schema URL and attributes together.
	ScopeName      string
	ScopeVersion   string
	ScopeSchemaURL string
	ScopeAttrs     string
	MetricName     string
	Unit           string
	MetricType     writev2.Metadata_MetricType
	AttrsHash      [16]byte // hash of data labels (excludes job, instance, __name__, otel_scope_*)
}

// scopeAttrsKey encodes scope attributes so that two scopes carrying different ones do not share
// exemplars. The attributes come from ranging over sorted labels, so the order is stable.
func scopeAttrsKey(attrs []attribute) string {
	if len(attrs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, a := range attrs {
		b.WriteString(a.Key)
		b.Write(sep)
		b.WriteString(a.Value)
		b.Write(sep)
	}
	return b.String()
}

// sep is a byte that is not valid UTF-8, used as a field separator to prevent
// hash collisions between different field boundary combinations (e.g. "ab"+"c" vs "a"+"bc").
var sep = []byte{0xff}

func (k exemplarKey) hash() uint64 {
	h := identity.Resource{}.Hash()
	h.Write([]byte(k.Job))
	h.Write(sep)
	h.Write([]byte(k.Instance))
	h.Write(sep)
	h.Write([]byte(k.ScopeName))
	h.Write(sep)
	h.Write([]byte(k.ScopeVersion))
	h.Write(sep)
	h.Write([]byte(k.ScopeSchemaURL))
	h.Write(sep)
	h.Write([]byte(k.ScopeAttrs))
	h.Write(sep)
	h.Write([]byte(k.MetricName))
	h.Write(sep)
	h.Write([]byte(k.Unit))
	h.Write(sep)
	h.Write([]byte(k.MetricType.String()))
	h.Write(sep)
	h.Write(k.AttrsHash[:])
	return h.Sum64()
}
