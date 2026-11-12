// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/schema"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// collectExemplars extracts Prometheus exemplars from a writev2 request and
// groups them into ExemplarSlices keyed by the series that carried them.
//
// TODO:
//
//	Right now, remote-write 2.0 sends disconnected exemplars without histogram, which requires
//	caching exemplars and associating them later with histogram data points.
//	Once https://github.com/prometheus/prometheus/issues/17857 is resolved, we can optimize this
func collectExemplars(
	req *writev2.Request,
	settings receiver.Settings,
) (map[exemplarKey]pmetric.ExemplarSlice, error) {
	var errs error
	result := make(map[exemplarKey]pmetric.ExemplarSlice)
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

		// A series that validateSeriesLabels would refuse is skipped here: its exemplars are refused
		// with the series, and the label lookups below assume a readable label set.
		if validateSeriesLabels(ls) != nil {
			continue
		}

		metadata := schema.NewMetadataFromLabels(ls)
		if metadata.Name == "" {
			settings.Logger.Warn("missing metric name in labels")
			continue
		}
		if ts.Metadata.UnitRef >= uint32(len(req.Symbols)) || ts.Metadata.HelpRef >= uint32(len(req.Symbols)) {
			continue
		}
		if metadata.Name != "target_info" {
			switch ts.Metadata.Type {
			case writev2.Metadata_METRIC_TYPE_GAUGE,
				writev2.Metadata_METRIC_TYPE_UNSPECIFIED,
				writev2.Metadata_METRIC_TYPE_COUNTER,
				writev2.Metadata_METRIC_TYPE_INFO,
				writev2.Metadata_METRIC_TYPE_STATESET,
				writev2.Metadata_METRIC_TYPE_SUMMARY,
				writev2.Metadata_METRIC_TYPE_HISTOGRAM:
			default:
				continue
			}
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
				errs = errors.Join(errs, fmt.Errorf("error converting exemplar label refs for metric %q: %w", metadata.Name, err))
				continue
			}

			exemplar := slice.AppendEmpty()
			exemplar.SetTimestamp(pcommon.Timestamp(ex.Timestamp * int64(time.Millisecond)))
			exemplar.SetDoubleValue(ex.Value)

			traceIDSet, spanIDSet := setTraceAndSpan(exemplar, promExemplar.Labels)
			copyExemplarAttributes(exemplar.FilteredAttributes(), promExemplar.Labels, traceIDSet, spanIDSet)
		}

		result[key] = slice
	}

	return result, errs
}

// setTraceAndSpan converts the hex-encoded trace and span ID labels and reports
// which ones it consumed, so the caller can keep a rejected value as a filtered
// attribute.
//
// Only valid IDs are converted: the label must decode to exactly the
// OpenTelemetry ID width and must not be all zero. Anything else is left for the
// caller rather than zero padded or truncated.
//
// One Range pass reads both values, which also keeps the lookup away from labels.Get: that
// indexes into the first byte of every stored name and is not safe on a set where one is empty.
// A name that repeats has no defined order in the set, so which of the repeated values is used
// depends on the order the sender wrote them in.
func setTraceAndSpan(exemplar pmetric.Exemplar, ls labels.Labels) (traceIDSet, spanIDSet bool) {
	var traceIDValue, spanIDValue string
	ls.Range(func(l labels.Label) {
		switch l.Name {
		case prometheus.ExemplarTraceIDKey:
			traceIDValue = l.Value
		case prometheus.ExemplarSpanIDKey:
			spanIDValue = l.Value
		}
	})

	var t [16]byte
	if len(traceIDValue) == hex.EncodedLen(len(t)) {
		if b, err := hex.DecodeString(traceIDValue); err == nil {
			copy(t[:], b)
			// all-zero is the "unset" sentinel: setting it would drop the label for nothing
			if traceID := pcommon.TraceID(t); !traceID.IsEmpty() {
				exemplar.SetTraceID(traceID)
				traceIDSet = true
			}
		}
	}
	var s [8]byte
	if len(spanIDValue) == hex.EncodedLen(len(s)) {
		if b, err := hex.DecodeString(spanIDValue); err == nil {
			copy(s[:], b)
			if spanID := pcommon.SpanID(s); !spanID.IsEmpty() {
				exemplar.SetSpanID(spanID)
				spanIDSet = true
			}
		}
	}
	return traceIDSet, spanIDSet
}

// copyExemplarAttributes copies the labels that can become attributes into dest, skipping the
// trace and span ID labels only when setTraceAndSpan converted them. A rejected trace or span ID
// label is copied like any other, so the value the sender wrote is not lost. Labels
// with an empty value are dropped, as the Prometheus data model equates an empty label
// with an absent one; the Prometheus receiver drops them from exemplars too.
//
// A label with an empty name is dropped as well: it cannot become an attribute key, and
// an exemplar is an annotation, so failing the request would cost the samples around it
// for one unusable label. Series labels are refused instead, since there the label set
// is the series identity (see validateSeriesLabels).
func copyExemplarAttributes(dest pcommon.Map, ls labels.Labels, traceIDSet, spanIDSet bool) {
	ls.Range(func(l labels.Label) {
		if l.Name == "" || l.Value == "" {
			return
		}
		if l.Name == prometheus.ExemplarTraceIDKey && traceIDSet {
			return
		}
		if l.Name == prometheus.ExemplarSpanIDKey && spanIDSet {
			return
		}
		dest.PutStr(l.Name, l.Value)
	})
}

// exemplarKey identifies a series the way Prometheus does, by its whole label set.
type exemplarKey string

// makeExemplarKey returns the map key for a series. ToLabels sorts its output, so the encoding is
// the same whatever order the labels arrived in, and the map does not outlive the request.
func makeExemplarKey(ls labels.Labels) exemplarKey {
	return exemplarKey(ls.Bytes(nil))
}
