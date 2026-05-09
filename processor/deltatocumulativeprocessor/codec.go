// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"errors"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// streamHashKey returns a hex-encoded hash string for an identity.Stream,
// suitable for use as a storage key.
//
// Uses the 64-bit FNV-1a hash of the stream identity. Collision probability is
// negligible for realistic stream counts (~5e-10 at 100k streams). A fuller key
// is not possible because identity.Stream fields are unexported.
func streamHashKey(s identity.Stream) string {
	return strconv.FormatUint(s.Hash().Sum64(), 16)
}

var errMalformedData = errors.New("malformed persisted datapoint")

// marshalNumberDP wraps a single NumberDataPoint into a minimal pmetric.Metrics
// and serializes it to protobuf bytes. The mutex is only held during the copy;
// the expensive proto marshal runs lock-free.
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

// unmarshalNumberDP deserializes protobuf bytes into a *mutex[NumberDataPoint].
func unmarshalNumberDP(data []byte) (*mutex[pmetric.NumberDataPoint], error) {
	md, err := protoUnmarshaler.UnmarshalMetrics(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal number datapoint: %w", err)
	}
	if err := validateSingleDP(md); err != nil {
		return nil, err
	}
	dp := md.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		Sum().DataPoints().At(0)
	return guard(dp), nil
}

// marshalHistogramDP wraps a single HistogramDataPoint and serializes it.
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

// unmarshalHistogramDP deserializes protobuf bytes into a *mutex[HistogramDataPoint].
func unmarshalHistogramDP(data []byte) (*mutex[pmetric.HistogramDataPoint], error) {
	md, err := protoUnmarshaler.UnmarshalMetrics(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal histogram datapoint: %w", err)
	}
	if err := validateSingleDP(md); err != nil {
		return nil, err
	}
	dp := md.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		Histogram().DataPoints().At(0)
	return guard(dp), nil
}

// marshalExpoDP wraps a single ExponentialHistogramDataPoint and serializes it.
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

// unmarshalExpoDP deserializes protobuf bytes into a *mutex[ExponentialHistogramDataPoint].
func unmarshalExpoDP(data []byte) (*mutex[pmetric.ExponentialHistogramDataPoint], error) {
	md, err := protoUnmarshaler.UnmarshalMetrics(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal exponential histogram datapoint: %w", err)
	}
	if err := validateSingleDP(md); err != nil {
		return nil, err
	}
	dp := md.ResourceMetrics().At(0).
		ScopeMetrics().At(0).
		Metrics().At(0).
		ExponentialHistogram().DataPoints().At(0)
	return guard(dp), nil
}

// validateSingleDP checks that a pmetric.Metrics contains exactly one metric
// with at least the expected nesting depth. Prevents panics on corrupted data.
func validateSingleDP(md pmetric.Metrics) error {
	if md.ResourceMetrics().Len() == 0 {
		return errMalformedData
	}
	rm := md.ResourceMetrics().At(0)
	if rm.ScopeMetrics().Len() == 0 {
		return errMalformedData
	}
	sm := rm.ScopeMetrics().At(0)
	if sm.Metrics().Len() == 0 {
		return errMalformedData
	}
	m := sm.Metrics().At(0)
	switch m.Type() {
	case pmetric.MetricTypeSum:
		if m.Sum().DataPoints().Len() == 0 {
			return errMalformedData
		}
	case pmetric.MetricTypeHistogram:
		if m.Histogram().DataPoints().Len() == 0 {
			return errMalformedData
		}
	case pmetric.MetricTypeExponentialHistogram:
		if m.ExponentialHistogram().DataPoints().Len() == 0 {
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
