// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/schema"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/zapcore"
)

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
			settings.Logger.Warn("error converting labels for exemplars", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
			continue
		}

		metadata := schema.NewMetadataFromLabels(ls)
		if metadata.Name == "" {
			continue
		}

		scopeName, scopeVersion := extractScopeFromLabels(settings, ls)

		key := exemplarKey{
			ScopeName:    scopeName,
			ScopeVersion: scopeVersion,
			MetricName:   metadata.Name,
			MetricType:   ts.Metadata.Type,
		}

		slice, ok := result[key.Hash()]
		if !ok {
			slice = pmetric.NewExemplarSlice()
		}

		for _, ex := range ts.Exemplars {
			labels, err := labelrefsToLabels(ex.LabelsRefs, req.Symbols)
			if err != nil {
				settings.Logger.Warn("error converting exemplar label refs", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
				continue
			}

			exemplar := slice.AppendEmpty()
			exemplar.SetTimestamp(pcommon.Timestamp(ex.Timestamp * int64(time.Millisecond)))
			exemplar.SetDoubleValue(ex.Value)

			setTraceAndSpan(exemplar, labels)
			copyExemplarAttributes(exemplar.FilteredAttributes(), labels)
			stats.Exemplars++
		}

		result[key.Hash()] = slice
	}

	return result
}

func extractScopeFromLabels(settings receiver.Settings, ls labels.Labels) (string, string) {
	name := settings.BuildInfo.Description
	version := settings.BuildInfo.Version

	if v := ls.Get("otel_scope_name"); v != "" {
		name = v
	}
	if v := ls.Get("otel_scope_version"); v != "" {
		version = v
	}
	return name, version
}

func setTraceAndSpan(exemplar pmetric.Exemplar, labels labels.Labels) {
	if tid := labels.Get(prometheus.ExemplarTraceIDKey); tid != "" {
		var t [16]byte
		if b, err := hex.DecodeString(tid); err == nil {
			copy(t[:], b)
			exemplar.SetTraceID(pcommon.TraceID(t))
		}
	}
	if sid := labels.Get(prometheus.ExemplarSpanIDKey); sid != "" {
		var s [8]byte
		if b, err := hex.DecodeString(sid); err == nil {
			copy(s[:], b)
			exemplar.SetSpanID(pcommon.SpanID(s))
		}
	}
}

func copyExemplarAttributes(dest pcommon.Map, labels labels.Labels) {
	for k, v := range labels.Map() {
		if k == prometheus.ExemplarTraceIDKey || k == prometheus.ExemplarSpanIDKey {
			continue
		}
		dest.PutStr(k, v)
	}
}

func labelrefsToLabels(labelRefs []uint32, symbols []string) (labels.Labels, error) {
	if len(labelRefs)%2 != 0 {
		return labels.EmptyLabels(), fmt.Errorf("invalid labelRefs length %d", len(labelRefs))
	}
	builder := labels.NewScratchBuilder(0)
	for i := 0; i < len(labelRefs); i += 2 {
		nameRef, valueRef := labelRefs[i], labelRefs[i+1]
		if int(nameRef) >= len(symbols) || int(valueRef) >= len(symbols) {
			return labels.EmptyLabels(), fmt.Errorf("labelRefs %d (name) = %d (value) outside of symbols table (size %d)", nameRef, valueRef, len(symbols))
		}
		builder.Add(symbols[nameRef], symbols[valueRef])
	}
	builder.Sort()
	return builder.Labels(), nil
}

type exemplarKey struct {
	ScopeName    string
	ScopeVersion string
	MetricName   string
	MetricType   writev2.Metadata_MetricType
}

func (k exemplarKey) Hash() uint64 {
	const sep = "\xff"
	return xxhash.Sum64String(strings.Join([]string{
		k.ScopeName,
		k.ScopeVersion,
		k.MetricName,
		fmt.Sprintf("%d", k.MetricType),
	}, sep))
}
