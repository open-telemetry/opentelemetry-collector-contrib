// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

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
	Value *ottl.ValueExpression[K]
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

// DynResAttrConfig groups the parsed dynamic resource attribute settings.
// A nil value means the feature is disabled for a given MetricDef.
type DynResAttrConfig[K any] struct {
	Expression *ottl.ValueExpression[K]
}

type MetricDef[K any] struct {
	Key                       MetricKey
	IncludeResourceAttributes []AttributeKeyValue
	Attributes                []AttributeKeyValue
	DynResAttrs               *DynResAttrConfig[K]
	Conditions                *ottl.ConditionSequence[K]
	ExponentialHistogram      *ExponentialHistogram[K]
	ExplicitHistogram         *ExplicitHistogram[K]
	Sum                       *Sum[K]
	Gauge                     *Gauge[K]
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
	if mi.DynamicResourceAttributes != nil && mi.DynamicResourceAttributes.Statement != "" {
		expr, dynErr := parser.ParseValueExpression(mi.DynamicResourceAttributes.Statement)
		if dynErr != nil {
			return fmt.Errorf("failed to parse dynamic_resource_attributes OTTL expression: %w", dynErr)
		}
		md.DynResAttrs = &DynResAttrConfig[K]{
			Expression: expr,
		}
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
//
// When DynamicResourceAttributes is set, the OTTL expression is evaluated
// and any resulting pcommon.Map entries are merged into the output.
func (md *MetricDef[K]) FilterResourceAttributes(
	ctx context.Context,
	tCtx K,
	attrs pcommon.Map,
	collectorInfo CollectorInstanceInfo,
	logger *zap.Logger,
) (pcommon.Map, error) {
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
	if dra := md.DynResAttrs; dra != nil {
		result, err := dra.Expression.Eval(ctx, tCtx)
		if err != nil {
			return pcommon.Map{}, fmt.Errorf("evaluating dynamic_resource_attributes expression: %w", err)
		}
		dynMap, ok := result.(pcommon.Map)
		if !ok {
			return pcommon.Map{}, fmt.Errorf(
				"dynamic_resource_attributes must return a pcommon.Map, got %T", result,
			)
		}

		dynMap.Range(func(k string, v pcommon.Value) bool {
			v.CopyTo(filteredAttributes.PutEmpty(k))
			return true
		})
	}
	collectorInfo.Copy(filteredAttributes)
	return filteredAttributes, nil
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
