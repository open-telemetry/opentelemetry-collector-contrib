// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"

import (
	"errors"
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type AttributeKeyValue struct {
	Key          string
	Optional     bool
	DefaultValue pcommon.Value
}

type MetricKey struct {
	Name        string
	Type        pmetric.MetricType
	Unit        string
	Description string
}

type ExplicitHistogram[K any] struct {
	Buckets []float64
	Count   *ottl.ValueExpression[K]
	Value   *ottl.ValueExpression[K]
}

func (h *ExplicitHistogram[K]) fromConfig(
	mi *config.Histogram,
	parser ottl.Parser[K],
) error {
	if mi == nil {
		return nil
	}

	var err error
	h.Buckets = mi.Buckets
	if mi.Count != "" {
		h.Count, err = parser.ParseValueExpression(mi.Count)
		if err != nil {
			return fmt.Errorf("failed to parse count OTTL expression for explicit histogram: %w", err)
		}
	}
	h.Value, err = parser.ParseValueExpression(mi.Value)
	if err != nil {
		return fmt.Errorf("failed to parse value statement for explicit histogram: %w", err)
	}
	return nil
}

type ExponentialHistogram[K any] struct {
	MaxSize int32
	Count   *ottl.ValueExpression[K]
	Value   *ottl.ValueExpression[K]
}

func (h *ExponentialHistogram[K]) fromConfig(
	mi *config.ExponentialHistogram,
	parser ottl.Parser[K],
) error {
	if mi == nil {
		return nil
	}

	var err error
	h.MaxSize = mi.MaxSize
	if mi.Count != "" {
		h.Count, err = parser.ParseValueExpression(mi.Count)
		if err != nil {
			return fmt.Errorf("failed to parse count OTTL expression for exponential histogram: %w", err)
		}
	}
	h.Value, err = parser.ParseValueExpression(mi.Value)
	if err != nil {
		return fmt.Errorf("failed to parse value OTTL expression for exponential histogram: %w", err)
	}
	return nil
}

type Sum[K any] struct {
	Value       *ottl.ValueExpression[K]
	IsMonotonic bool
}

func (s *Sum[K]) fromConfig(
	mi *config.Sum,
	parser ottl.Parser[K],
) error {
	if mi == nil {
		return nil
	}

	var err error
	s.Value, err = parser.ParseValueExpression(mi.Value)
	if err != nil {
		return fmt.Errorf("failed to parse value OTTL expression for sum: %w", err)
	}
	s.IsMonotonic = mi.IsMonotonic
	return nil
}

type Gauge[K any] struct {
	Value *ottl.ValueExpression[K]
}

func (s *Gauge[K]) fromConfig(
	mi *config.Gauge,
	parser ottl.Parser[K],
) error {
	if mi == nil {
		return nil
	}

	var err error
	s.Value, err = parser.ParseValueExpression(mi.Value)
	if err != nil {
		return fmt.Errorf("failed to parse value OTTL expression for gauge: %w", err)
	}
	return nil
}

type MetricDef[K any] struct {
	Key                       MetricKey
	IncludeResourceAttributes []AttributeKeyValue
	Attributes                []AttributeKeyValue
	// sortedAttrs is Attributes sorted alphabetically by key, used for
	// deterministic hash computation in FilterAttributesID.
	sortedAttrs          []AttributeKeyValue
	Conditions           *ottl.ConditionSequence[K]
	ExponentialHistogram *ExponentialHistogram[K]
	ExplicitHistogram    *ExplicitHistogram[K]
	Sum                  *Sum[K]
	Gauge                *Gauge[K]
}

func (md *MetricDef[K]) FromMetricInfo(
	mi config.MetricInfo,
	parser ottl.Parser[K],
	telemetrySettings component.TelemetrySettings,
) error {
	md.Key.Name = mi.Name
	md.Key.Unit = mi.Unit
	md.Key.Description = mi.Description

	var err error
	md.IncludeResourceAttributes, err = parseAttributeConfigs(mi.IncludeResourceAttributes)
	if err != nil {
		return fmt.Errorf("failed to parse include resource attribute config: %w", err)
	}
	md.Attributes, err = parseAttributeConfigs(mi.Attributes)
	if err != nil {
		return fmt.Errorf("failed to parse attribute config: %w", err)
	}
	// Pre-sort a copy of Attributes by key for deterministic hash computation
	// in FilterAttributesID.
	if len(md.Attributes) > 0 {
		md.sortedAttrs = make([]AttributeKeyValue, len(md.Attributes))
		copy(md.sortedAttrs, md.Attributes)
		sort.Slice(md.sortedAttrs, func(i, j int) bool {
			return md.sortedAttrs[i].Key < md.sortedAttrs[j].Key
		})
	}
	if len(mi.Conditions) > 0 {
		conditions, err := parser.ParseConditions(mi.Conditions)
		if err != nil {
			return fmt.Errorf("failed to parse OTTL conditions: %w", err)
		}
		condSeq := ottl.NewConditionSequence(
			conditions,
			telemetrySettings,
			ottl.WithLogicOperation[K](ottl.Or),
		)
		md.Conditions = &condSeq
	}
	if mi.Histogram.HasValue() {
		md.Key.Type = pmetric.MetricTypeHistogram
		md.ExplicitHistogram = new(ExplicitHistogram[K])
		if err := md.ExplicitHistogram.fromConfig(mi.Histogram.Get(), parser); err != nil {
			return fmt.Errorf("failed to parse histogram config: %w", err)
		}
	}
	if mi.ExponentialHistogram.HasValue() {
		md.Key.Type = pmetric.MetricTypeExponentialHistogram
		md.ExponentialHistogram = new(ExponentialHistogram[K])
		if err := md.ExponentialHistogram.fromConfig(mi.ExponentialHistogram.Get(), parser); err != nil {
			return fmt.Errorf("failed to parse histogram config: %w", err)
		}
	}
	if mi.Sum.HasValue() {
		md.Key.Type = pmetric.MetricTypeSum
		md.Sum = new(Sum[K])
		if err := md.Sum.fromConfig(mi.Sum.Get(), parser); err != nil {
			return fmt.Errorf("failed to parse sum config: %w", err)
		}
	}
	if mi.Gauge.HasValue() {
		md.Key.Type = pmetric.MetricTypeGauge
		md.Gauge = new(Gauge[K])
		if err := md.Gauge.fromConfig(mi.Gauge.Get(), parser); err != nil {
			return fmt.Errorf("failed to parse gauge config: %w", err)
		}
	}
	return nil
}

// FilterResourceAttributes filters resource attributes based on the
// `IncludeResourceAttributes` list for the metric definition. Resource
// attributes are only filtered if the list is specified, otherwise all the
// resource attributes are used for creating the metrics from the metric
// definition.
func (md *MetricDef[K]) FilterResourceAttributes(
	attrs pcommon.Map,
	collectorInfo CollectorInstanceInfo,
) pcommon.Map {
	var filteredAttributes pcommon.Map
	switch {
	case len(md.IncludeResourceAttributes) == 0:
		filteredAttributes = pcommon.NewMap()
		filteredAttributes.EnsureCapacity(attrs.Len() + collectorInfo.Size())
		attrs.CopyTo(filteredAttributes)
	default:
		expectedLen := len(md.IncludeResourceAttributes) + collectorInfo.Size()
		filteredAttributes = filterAttributes(attrs, md.IncludeResourceAttributes, expectedLen)
	}
	collectorInfo.Copy(filteredAttributes)
	return filteredAttributes
}

// FilterAttributes filters event attributes (datapoint, logrecord, spans)
// based on the `Attributes` selected for the metric definition. If no
// attributes are selected then an empty `pcommon.Map` is returned. Note
// that, this filtering differs from resource attribute filtering as
// in attribute filtering if any of the configured attributes is not present
// in the data being processed then that metric definition is not processed.
// The method returns a bool signaling if the filter was successful and metric
// should be processed. If the bool value is false then the returned map
// should not be used.
func (md *MetricDef[K]) FilterAttributes(attrs pcommon.Map) (pcommon.Map, bool) {
	// Figure out if all the attributes are available, saves allocation
	for _, filter := range md.Attributes {
		if filter.DefaultValue.Type() != pcommon.ValueTypeEmpty || filter.Optional {
			continue
		}
		if _, ok := attrs.Get(filter.Key); !ok {
			return pcommon.Map{}, false
		}
	}
	return filterAttributes(attrs, md.Attributes, len(md.Attributes)), true
}

// FilterAttributesID returns a 128-bit hash that identifies the filtered
// attribute set for the given source attributes, without allocating a
// pcommon.Map. The returned hash is equivalent in purpose to
// pdatautil.MapHash applied to the result of FilterAttributes, but uses a
// different (internal-only) encoding. Returns false if any required attribute
// is absent.
//
// This is used in conjunction with FilterAttributesUnchecked: call
// FilterAttributesID first to check presence and compute the DP lookup key,
// then call FilterAttributesUnchecked only when a new DP must be created.
func (md *MetricDef[K]) FilterAttributesID(attrs pcommon.Map) ([16]byte, bool) {
	for _, filter := range md.Attributes {
		if filter.DefaultValue.Type() != pcommon.ValueTypeEmpty || filter.Optional {
			continue
		}
		if _, ok := attrs.Get(filter.Key); !ok {
			return [16]byte{}, false
		}
	}
	hb := attrHashBufPool.Get().(*attrHashBuf)
	hb.buf = hb.buf[:0]
	for _, filter := range md.sortedAttrs {
		v, ok := attrs.Get(filter.Key)
		if ok {
			hb.buf = append(hb.buf, filter.Key...)
			hb.buf = append(hb.buf, 0) // key-value separator
			hb.buf = appendAttrValue(hb.buf, v)
		} else if filter.DefaultValue.Type() != pcommon.ValueTypeEmpty {
			hb.buf = append(hb.buf, filter.Key...)
			hb.buf = append(hb.buf, 0)
			hb.buf = appendAttrValue(hb.buf, filter.DefaultValue)
		}
		// Optional key absent: not included in hash
	}
	id := hb.sum128()
	attrHashBufPool.Put(hb)
	return id, true
}

// FilterAttributesUnchecked creates a filtered pcommon.Map from attrs without
// re-checking for required attribute presence. It must only be called after
// FilterAttributesID has returned true for the same attrs.
func (md *MetricDef[K]) FilterAttributesUnchecked(attrs pcommon.Map) pcommon.Map {
	return filterAttributes(attrs, md.Attributes, len(md.Attributes))
}

func filterAttributes(attrs pcommon.Map, filters []AttributeKeyValue, expectedLen int) pcommon.Map {
	filteredAttrs := pcommon.NewMap()
	filteredAttrs.EnsureCapacity(expectedLen)
	for _, filter := range filters {
		if attr, ok := attrs.Get(filter.Key); ok {
			attr.CopyTo(filteredAttrs.PutEmpty(filter.Key))
			continue
		}
		if filter.DefaultValue.Type() != pcommon.ValueTypeEmpty {
			filter.DefaultValue.CopyTo(filteredAttrs.PutEmpty(filter.Key))
		}
	}
	return filteredAttrs
}

func parseAttributeConfigs(cfgs []config.Attribute) ([]AttributeKeyValue, error) {
	var errs []error
	kvs := make([]AttributeKeyValue, len(cfgs))
	for i, attr := range cfgs {
		val := pcommon.NewValueEmpty()
		if err := val.FromRaw(attr.DefaultValue); err != nil {
			errs = append(errs, err)
		}
		kvs[i] = AttributeKeyValue{
			Key:          attr.Key,
			Optional:     attr.Optional,
			DefaultValue: val,
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return kvs, nil
}
