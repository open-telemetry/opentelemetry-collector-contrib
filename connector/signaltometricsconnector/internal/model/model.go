// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

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

// AttributeEntry represents a single entry in include_resource_attributes
// or attributes. Exactly one of Key or Expression must be set.
type AttributeEntry[K any] struct {
	// Key is a static attribute key.
	Key string
	// Expression is a parsed OTTL value expression that resolves to a
	// list of attribute keys (pcommon.Slice or []string) at runtime.
	Expression   *ottl.ValueExpression[K]
	Optional     bool
	DefaultValue pcommon.Value
}

// AttributeKeyValue represents a static-key attribute entry used for
// deterministic hash computation and unchecked filtering. It does not
// support OTTL expressions.
type AttributeKeyValue struct {
	Key          string
	Optional     bool
	DefaultValue pcommon.Value
}

type MetricDef[K any] struct {
	Key                       MetricKey
	IncludeResourceAttributes []AttributeEntry[K]
	Attributes                []AttributeEntry[K]
	// sortedAttrs is the static-key subset of Attributes sorted
	// alphabetically by key, used for deterministic hash computation
	// in FilterAttributesID.
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
	md.IncludeResourceAttributes, err = parseAttributeEntries(mi.IncludeResourceAttributes, parser)
	if err != nil {
		return fmt.Errorf("failed to parse include resource attribute config: %w", err)
	}
	md.Attributes, err = parseAttributeEntries(mi.Attributes, parser)
	if err != nil {
		return fmt.Errorf("failed to parse attribute config: %w", err)
	}
	// Pre-sort a copy of static-key Attributes by key for deterministic hash
	// computation in FilterAttributesID. OTTL expression entries are excluded
	// because they are resolved at runtime.
	staticEntries := make([]AttributeKeyValue, 0, len(md.Attributes))
	for _, entry := range md.Attributes {
		if entry.Expression != nil {
			continue
		}
		staticEntries = append(staticEntries, AttributeKeyValue{
			Key:          entry.Key,
			Optional:     entry.Optional,
			DefaultValue: entry.DefaultValue,
		})
	}
	if len(staticEntries) > 0 {
		md.sortedAttrs = staticEntries
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
//
// For entries with an OTTL expression, the expression is evaluated at
// runtime to resolve a list of attribute keys. The expression must return
// a pcommon.Slice or []string. A nil result is treated as an empty list.
func (md *MetricDef[K]) FilterResourceAttributes(
	ctx context.Context,
	tCtx K,
	attrs pcommon.Map,
	collectorInfo CollectorInstanceInfo,
) (pcommon.Map, error) {
	if len(md.IncludeResourceAttributes) == 0 {
		filteredAttributes := pcommon.NewMap()
		filteredAttributes.EnsureCapacity(attrs.Len() + collectorInfo.Size())
		attrs.CopyTo(filteredAttributes)
		collectorInfo.Copy(filteredAttributes)
		return filteredAttributes, nil
	}

	filteredAttributes, err := filterByEntries(ctx, tCtx, md.IncludeResourceAttributes, attrs, collectorInfo.Size())
	if err != nil {
		return pcommon.Map{}, err
	}
	collectorInfo.Copy(filteredAttributes)
	return filteredAttributes, nil
}

// MatchAttributes checks if all required static key attributes are
// present in the given map. This is a cheap pre-check that does not
// require a transform context and can be used to skip expensive
// transform context creation when the entity should not be processed.
// OTTL expression entries and entries with default values or optional
// flag are not checked.
func (md *MetricDef[K]) MatchAttributes(attrs pcommon.Map) bool {
	for _, filter := range md.Attributes {
		if filter.Expression != nil || filter.DefaultValue.Type() != pcommon.ValueTypeEmpty || filter.Optional {
			continue
		}
		if _, ok := attrs.Get(filter.Key); !ok {
			return false
		}
	}
	return true
}

// FilterAttributesID returns a 128-bit hash that identifies the filtered
// attribute set for the given source attributes, without allocating a
// pcommon.Map. The returned hash is equivalent in purpose to
// pdatautil.MapHash applied to the result of FilterAttributes, but uses a
// different (internal-only) encoding. Returns false if any required
// static-key attribute is absent.
//
// This only considers static-key entries in Attributes (not OTTL
// expression entries). It is used in conjunction with
// FilterAttributesUnchecked: call FilterAttributesID first to check
// presence and compute the DP lookup key, then call
// FilterAttributesUnchecked only when a new DP must be created.
func (md *MetricDef[K]) FilterAttributesID(attrs pcommon.Map) ([16]byte, bool) {
	for _, filter := range md.Attributes {
		if filter.Expression != nil || filter.DefaultValue.Type() != pcommon.ValueTypeEmpty || filter.Optional {
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

// FilterAttributesUnchecked creates a filtered pcommon.Map from attrs
// using only the static-key entries in Attributes, without re-checking
// for required attribute presence. It must only be called after
// FilterAttributesID has returned true for the same attrs.
func (md *MetricDef[K]) FilterAttributesUnchecked(attrs pcommon.Map) pcommon.Map {
	return filterAttributes(attrs, md.sortedAttrs, len(md.sortedAttrs))
}

// FilterAttributes filters event attributes (datapoint, logrecord, spans)
// based on the `Attributes` selected for the metric definition. If no
// attributes are selected then an empty `pcommon.Map` is returned.
// MatchAttributes should be called before this method to avoid unnecessary
// transform context creation.
func (md *MetricDef[K]) FilterAttributes(ctx context.Context, tCtx K, attrs pcommon.Map) (pcommon.Map, error) {
	return filterByEntries(ctx, tCtx, md.Attributes, attrs, 0)
}

// filterByEntries iterates over attribute entries, resolving OTTL
// expressions and copying static keys from src into a new map.
// extraCapacity is added to the initial map capacity for entries
// like CollectorInstanceInfo that are appended after filtering.
func filterByEntries[K any](
	ctx context.Context,
	tCtx K,
	entries []AttributeEntry[K],
	src pcommon.Map,
	extraCapacity int,
) (pcommon.Map, error) {
	dst := pcommon.NewMap()
	dst.EnsureCapacity(len(entries) + extraCapacity)
	for _, entry := range entries {
		if entry.Expression != nil {
			if err := resolveKeysExpression(ctx, tCtx, entry, src, dst); err != nil {
				return pcommon.Map{}, err
			}
			continue
		}
		copyAttribute(entry.Key, entry.DefaultValue, src, dst)
	}
	return dst, nil
}

// filterAttributes creates a filtered pcommon.Map from attrs using only
// static-key AttributeKeyValue entries. This is used by
// FilterAttributesUnchecked for the lazy-allocation path.
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

func parseAttributeEntries[K any](
	cfgs []config.Attribute,
	parser ottl.Parser[K],
) ([]AttributeEntry[K], error) {
	var errs []error
	entries := make([]AttributeEntry[K], 0, len(cfgs))
	for _, attr := range cfgs {
		val := pcommon.NewValueEmpty()
		if err := val.FromRaw(attr.DefaultValue); err != nil {
			errs = append(errs, err)
			continue
		}
		entry := AttributeEntry[K]{
			Optional:     attr.Optional,
			DefaultValue: val,
		}
		if attr.KeysExpression != "" {
			expr, err := parser.ParseValueExpression(attr.KeysExpression)
			if err != nil {
				errs = append(errs, fmt.Errorf(
					"failed to parse keys_expression %q: %w", attr.KeysExpression, err,
				))
				continue
			}
			entry.Expression = expr
		} else {
			entry.Key = attr.Key
		}
		entries = append(entries, entry)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return entries, nil
}

// resolveKeysExpression evaluates the OTTL expression to get a list of
// attribute keys and copies matching attributes from src to dst.
func resolveKeysExpression[K any](
	ctx context.Context,
	tCtx K,
	entry AttributeEntry[K],
	src, dst pcommon.Map,
) error {
	result, err := entry.Expression.Eval(ctx, tCtx)
	if err != nil {
		return fmt.Errorf("failed to evaluate keys_expression: %w", err)
	}
	if result == nil {
		return nil
	}
	keys, err := extractStringSlice(result)
	if err != nil {
		return fmt.Errorf("keys_expression must return a list of strings: %w", err)
	}
	for _, key := range keys {
		copyAttribute(key, entry.DefaultValue, src, dst)
	}
	return nil
}

func copyAttribute(key string, defaultValue pcommon.Value, src, dst pcommon.Map) {
	if attr, ok := src.Get(key); ok {
		attr.CopyTo(dst.PutEmpty(key))
		return
	}
	if defaultValue.Type() != pcommon.ValueTypeEmpty {
		defaultValue.CopyTo(dst.PutEmpty(key))
	}
}

func extractStringSlice(val any) ([]string, error) {
	switch v := val.(type) {
	case pcommon.Slice:
		keys := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			elem := v.At(i)
			if elem.Type() != pcommon.ValueTypeStr {
				return nil, fmt.Errorf("expected string element at index %d, got %s", i, elem.Type())
			}
			keys = append(keys, elem.Str())
		}
		return keys, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type %T, expected pcommon.Slice or []string", val)
	}
}
