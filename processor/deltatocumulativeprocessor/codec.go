// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
)

// Bump for incompatible changes to a released key or datapoint encoding.
const storageKeyVersion = "v1"

type streamID struct {
	identity.Stream
	fingerprint [sha256.Size]byte
}

func streamHashKey(s streamID) string {
	return storageKeyVersion + "/" + hex.EncodeToString(s.fingerprint[:])
}

func streamFingerprint(m metrics.Metric, attrs pcommon.Map) [sha256.Size]byte {
	encoded := make([]byte, 0, 64+len(m.Scope().Name())+len(m.Scope().Version())+len(m.Name())+len(m.Unit()))
	for _, attrs := range []pcommon.Map{m.Resource().Attributes(), m.Scope().Attributes(), attrs} {
		encoded = appendAttributeMap(encoded, attrs)
	}
	for _, field := range []string{m.Scope().Name(), m.Scope().Version(), m.Name(), m.Unit()} {
		encoded = appendKeyString(encoded, field)
	}
	encoded = binary.BigEndian.AppendUint32(encoded, uint32(m.Type()))
	encoded = binary.BigEndian.AppendUint32(encoded, uint32(m.AggregationTemporality()))
	var monotonic byte
	switch m.Type() {
	case pmetric.MetricTypeSum:
		if m.Sum().IsMonotonic() {
			monotonic = 1
		}
	case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram:
		monotonic = 1
	}
	encoded = append(encoded, monotonic)
	return sha256.Sum256(encoded)
}

// This encoding belongs to the storage format; it must not depend on pdata's in-memory hashes.
func appendAttributeMap(dst []byte, attrs pcommon.Map) []byte {
	type entry struct {
		key   string
		value pcommon.Value
	}
	entries := make([]entry, 0, attrs.Len())
	for key, value := range attrs.All() {
		entries = append(entries, entry{key: key, value: value})
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.key, b.key) })
	dst = binary.AppendUvarint(dst, uint64(len(entries)))
	for _, entry := range entries {
		dst = appendAttributeValue(appendKeyString(dst, entry.key), entry.value)
	}
	return dst
}

func appendKeyString(dst []byte, value string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func appendAttributeValue(dst []byte, value pcommon.Value) []byte {
	switch value.Type() {
	case pcommon.ValueTypeEmpty:
		return append(dst, 'n')
	case pcommon.ValueTypeStr:
		return appendKeyString(append(dst, 's'), value.Str())
	case pcommon.ValueTypeBool:
		var b byte
		if value.Bool() {
			b = 1
		}
		return append(dst, 'b', b)
	case pcommon.ValueTypeInt:
		return binary.BigEndian.AppendUint64(append(dst, 'i'), uint64(value.Int()))
	case pcommon.ValueTypeDouble:
		return binary.BigEndian.AppendUint64(append(dst, 'f'), math.Float64bits(value.Double()))
	case pcommon.ValueTypeBytes:
		data := value.Bytes().AsRaw()
		dst = binary.AppendUvarint(append(dst, 'x'), uint64(len(data)))
		return append(dst, data...)
	case pcommon.ValueTypeMap:
		return appendAttributeMap(append(dst, 'm'), value.Map())
	case pcommon.ValueTypeSlice:
		values := value.Slice()
		dst = binary.AppendUvarint(append(dst, 'a'), uint64(values.Len()))
		for i := 0; i < values.Len(); i++ {
			dst = appendAttributeValue(dst, values.At(i))
		}
		return dst
	}
	panic("unsupported attribute type")
}

var errMalformedData = errors.New("malformed persisted datapoint")

func marshalNumberDP(v *mutex[pmetric.NumberDataPoint]) ([]byte, error) {
	md := pmetric.NewMetrics()
	target := md.ResourceMetrics().AppendEmpty().
		ScopeMetrics().AppendEmpty().
		Metrics().AppendEmpty().
		SetEmptySum().DataPoints().AppendEmpty()
	v.use(func(dp pmetric.NumberDataPoint) {
		dp.CopyTo(target)
	})
	return protoMarshaler.MarshalMetrics(md)
}

func unmarshalNumberDP(data []byte) (*mutex[pmetric.NumberDataPoint], error) {
	md, err := protoUnmarshaler.UnmarshalMetrics(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal number datapoint: %w", err)
	}
	if err := validateSingleDP(md, pmetric.MetricTypeSum); err != nil {
		return nil, err
	}
	dp := md.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		Sum().DataPoints().At(0)
	return guard(dp), nil
}

func marshalHistogramDP(v *mutex[pmetric.HistogramDataPoint]) ([]byte, error) {
	md := pmetric.NewMetrics()
	target := md.ResourceMetrics().AppendEmpty().
		ScopeMetrics().AppendEmpty().
		Metrics().AppendEmpty().
		SetEmptyHistogram().DataPoints().AppendEmpty()
	v.use(func(dp pmetric.HistogramDataPoint) {
		dp.CopyTo(target)
	})
	return protoMarshaler.MarshalMetrics(md)
}

func unmarshalHistogramDP(data []byte) (*mutex[pmetric.HistogramDataPoint], error) {
	md, err := protoUnmarshaler.UnmarshalMetrics(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal histogram datapoint: %w", err)
	}
	if err := validateSingleDP(md, pmetric.MetricTypeHistogram); err != nil {
		return nil, err
	}
	dp := md.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		Histogram().DataPoints().At(0)
	return guard(dp), nil
}

func marshalExpoDP(v *mutex[pmetric.ExponentialHistogramDataPoint]) ([]byte, error) {
	md := pmetric.NewMetrics()
	target := md.ResourceMetrics().AppendEmpty().
		ScopeMetrics().AppendEmpty().
		Metrics().AppendEmpty().
		SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	v.use(func(dp pmetric.ExponentialHistogramDataPoint) {
		dp.CopyTo(target)
	})
	return protoMarshaler.MarshalMetrics(md)
}

func unmarshalExpoDP(data []byte) (*mutex[pmetric.ExponentialHistogramDataPoint], error) {
	md, err := protoUnmarshaler.UnmarshalMetrics(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal exponential histogram datapoint: %w", err)
	}
	if err := validateSingleDP(md, pmetric.MetricTypeExponentialHistogram); err != nil {
		return nil, err
	}
	dp := md.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		ExponentialHistogram().DataPoints().At(0)
	return guard(dp), nil
}

func validateSingleDP(md pmetric.Metrics, expected pmetric.MetricType) error {
	if md.ResourceMetrics().Len() != 1 {
		return errMalformedData
	}
	rm := md.ResourceMetrics().At(0)
	if rm.ScopeMetrics().Len() != 1 {
		return errMalformedData
	}
	sm := rm.ScopeMetrics().At(0)
	if sm.Metrics().Len() != 1 {
		return errMalformedData
	}
	m := sm.Metrics().At(0)
	if m.Type() != expected {
		return errMalformedData
	}
	switch m.Type() {
	case pmetric.MetricTypeSum:
		if m.Sum().DataPoints().Len() != 1 {
			return errMalformedData
		}
	case pmetric.MetricTypeHistogram:
		if m.Histogram().DataPoints().Len() != 1 {
			return errMalformedData
		}
	case pmetric.MetricTypeExponentialHistogram:
		if m.ExponentialHistogram().DataPoints().Len() != 1 {
			return errMalformedData
		}
	default:
		return errMalformedData
	}
	return nil
}

var (
	protoMarshaler   = &pmetric.ProtoMarshaler{}
	protoUnmarshaler = &pmetric.ProtoUnmarshaler{}
)
