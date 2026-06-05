// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

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
	pc *ottl.ParserCollection[*ottl.ValueExpression[K]],
	contextName string,
) error {
	if mi == nil {
		return nil
	}

	var err error
	h.Buckets = mi.Buckets
	if mi.Count != "" {
		h.Count, err = pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Count}), true)
		if err != nil {
			return fmt.Errorf("failed to parse count OTTL expression for explicit histogram: %w", err)
		}
	}
	h.Value, err = pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Value}), true)
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
	pc *ottl.ParserCollection[*ottl.ValueExpression[K]],
	contextName string,
) error {
	if mi == nil {
		return nil
	}

	var err error
	h.MaxSize = mi.MaxSize
	if mi.Count != "" {
		h.Count, err = pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Count}), true)
		if err != nil {
			return fmt.Errorf("failed to parse count OTTL expression for exponential histogram: %w", err)
		}
	}
	h.Value, err = pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Value}), true)
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
	pc *ottl.ParserCollection[*ottl.ValueExpression[K]],
	contextName string,
) error {
	if mi == nil {
		return nil
	}

	var err error
	s.Value, err = pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Value}), true)
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
	pc *ottl.ParserCollection[*ottl.ValueExpression[K]],
	contextName string,
) error {
	if mi == nil {
		return nil
	}

	var err error
	s.Value, err = pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Value}), true)
	if err != nil {
		return fmt.Errorf("failed to parse value OTTL expression for gauge: %w", err)
	}
	return nil
}

// attributeEntry represents a single entry in include_resource_attributes
// or attributes. Exactly one of Key or Expression must be set.
type attributeEntry[K any] struct {
	Key          string
	Expression   *ottl.ValueExpression[K]
	Optional     bool
	DefaultValue pcommon.Value
}

// AttributeKeyValue represents a resolved attribute entry with a static
// key. This is the output of ResolveAttributes and
// ResolveIncludeResourceAttributes after OTTL expressions have been
// evaluated and flattened into the final ordered list.
type AttributeKeyValue struct {
	Key          string
	Optional     bool
	DefaultValue pcommon.Value
}

type MetricDef[K any] struct {
	Key                       MetricKey
	includeResourceAttributes []attributeEntry[K]
	attributes                []attributeEntry[K]
	// hasExprResAttrs is true if any includeResourceAttributes entry
	// has a keys_expression. When false, the static-only fast path
	// skips OTTL evaluation and dedup overhead in
	// ResolveIncludeResourceAttributes.
	hasExprResAttrs bool
	// hasExprAttrs is true if any attributes entry has a
	// keys_expression. When false, the static-only fast path skips
	// OTTL evaluation and dedup overhead in ResolveAttributes, and
	// sortedStaticAttrs is used directly by ComputeAttributesHash
	// to avoid per-call sorting.
	hasExprAttrs bool
	// sortedStaticAttrs is a pre-sorted copy of the static-key
	// attributes, built at construction time. Used by
	// ResolveAttributes (static fast path) and ComputeAttributesHash
	// to avoid per-call allocation and sorting when there are no
	// keys_expression entries.
	sortedStaticAttrs []AttributeKeyValue
	// sortedStaticResAttrs is the same for includeResourceAttributes.
	sortedStaticResAttrs []AttributeKeyValue
	Conditions           *ottl.ConditionSequence[K]
	ExponentialHistogram *ExponentialHistogram[K]
	ExplicitHistogram    *ExplicitHistogram[K]
	Sum                  *Sum[K]
	Gauge                *Gauge[K]
}

func (md *MetricDef[K]) FromMetricInfo(
	mi config.MetricInfo,
	pc *ottl.ParserCollection[*ottl.ValueExpression[K]],
	contextName string,
	conditions *ottl.ConditionSequence[K],
) error {
	md.Key.Name = mi.Name
	md.Key.Unit = mi.Unit
	md.Key.Description = mi.Description

	var err error
	md.includeResourceAttributes, err = parseAttributeEntries(mi.IncludeResourceAttributes, pc, contextName)
	if err != nil {
		return fmt.Errorf("failed to parse include resource attribute config: %w", err)
	}
	md.attributes, err = parseAttributeEntries(mi.Attributes, pc, contextName)
	if err != nil {
		return fmt.Errorf("failed to parse attribute config: %w", err)
	}
	// Detect whether any entries use keys_expression and pre-build
	// sorted static attribute lists for the fast path.
	md.hasExprResAttrs, md.sortedStaticResAttrs = buildStaticAttrs(md.includeResourceAttributes)
	md.hasExprAttrs, md.sortedStaticAttrs = buildStaticAttrs(md.attributes)
	md.Conditions = conditions
	if mi.Histogram.HasValue() {
		md.Key.Type = pmetric.MetricTypeHistogram
		md.ExplicitHistogram = new(ExplicitHistogram[K])
		if err := md.ExplicitHistogram.fromConfig(mi.Histogram.Get(), pc, contextName); err != nil {
			return fmt.Errorf("failed to parse histogram config: %w", err)
		}
	}
	if mi.ExponentialHistogram.HasValue() {
		md.Key.Type = pmetric.MetricTypeExponentialHistogram
		md.ExponentialHistogram = new(ExponentialHistogram[K])
		if err := md.ExponentialHistogram.fromConfig(mi.ExponentialHistogram.Get(), pc, contextName); err != nil {
			return fmt.Errorf("failed to parse histogram config: %w", err)
		}
	}
	if mi.Sum.HasValue() {
		md.Key.Type = pmetric.MetricTypeSum
		md.Sum = new(Sum[K])
		if err := md.Sum.fromConfig(mi.Sum.Get(), pc, contextName); err != nil {
			return fmt.Errorf("failed to parse sum config: %w", err)
		}
	}
	if mi.Gauge.HasValue() {
		md.Key.Type = pmetric.MetricTypeGauge
		md.Gauge = new(Gauge[K])
		if err := md.Gauge.fromConfig(mi.Gauge.Get(), pc, contextName); err != nil {
			return fmt.Errorf("failed to parse gauge config: %w", err)
		}
	}
	return nil
}

// ResolveIncludeResourceAttributes evaluates OTTL expressions in
// include_resource_attributes and returns a flat, ordered list of
// AttributeKeyValue entries. When no keys_expression entries are
// configured, the pre-built sortedStaticResAttrs is returned directly
// without allocation or OTTL evaluation.
func (md *MetricDef[K]) ResolveIncludeResourceAttributes(ctx context.Context, tCtx K) ([]AttributeKeyValue, error) {
	if !md.hasExprResAttrs {
		return md.sortedStaticResAttrs, nil
	}
	return resolveEntries(ctx, tCtx, md.includeResourceAttributes)
}

// ResolveAttributes evaluates OTTL expressions in attributes and
// returns a flat, ordered list of AttributeKeyValue entries. When no
// keys_expression entries are configured, the pre-built
// sortedStaticAttrs is returned directly without allocation or OTTL
// evaluation.
func (md *MetricDef[K]) ResolveAttributes(ctx context.Context, tCtx K) ([]AttributeKeyValue, error) {
	if !md.hasExprAttrs {
		return md.sortedStaticAttrs, nil
	}
	return resolveEntries(ctx, tCtx, md.attributes)
}

// MatchAttributes checks if all required static key attributes are
// present in the given map. This is a cheap pre-check that does not
// require a transform context and can be used to skip expensive
// transform context creation when the entity should not be processed.
// OTTL expression entries and entries with default values or optional
// flag are not checked — the presence of an expression entry means
// there is a possibility of attributes resolving, so MatchAttributes
// will not reject the entity on that basis.
func (md *MetricDef[K]) MatchAttributes(attrs pcommon.Map) bool {
	for _, filter := range md.attributes {
		if filter.Expression != nil || filter.DefaultValue.Type() != pcommon.ValueTypeEmpty || filter.Optional {
			continue
		}
		if _, ok := attrs.Get(filter.Key); !ok {
			return false
		}
	}
	return true
}

// ComputeAttributesHash returns a 128-bit hash that identifies the
// filtered attribute set. The resolved list is expected to be sorted
// alphabetically by key (guaranteed by both resolveEntries and the
// pre-sorted sortedStaticAttrs).
func (*MetricDef[K]) ComputeAttributesHash(attrs pcommon.Map, resolved []AttributeKeyValue) [16]byte {
	hb := attrHashBufPool.Get().(*attrHashBuf)
	hb.buf = hb.buf[:0]
	for _, kv := range resolved {
		v, ok := attrs.Get(kv.Key)
		if ok {
			hb.buf = append(hb.buf, kv.Key...)
			hb.buf = append(hb.buf, 0)
			hb.buf = appendAttrValue(hb.buf, v)
		} else if kv.DefaultValue.Type() != pcommon.ValueTypeEmpty {
			hb.buf = append(hb.buf, kv.Key...)
			hb.buf = append(hb.buf, 0)
			hb.buf = appendAttrValue(hb.buf, kv.DefaultValue)
		}
	}
	id := hb.sum128()
	attrHashBufPool.Put(hb)
	return id
}

// FilterResourceAttributes builds a pcommon.Map from the resolved
// include_resource_attributes list. If the list is empty, all resource
// attributes are copied. CollectorInstanceInfo is always appended.
func (md *MetricDef[K]) FilterResourceAttributes(
	attrs pcommon.Map,
	resolved []AttributeKeyValue,
	collectorInfo CollectorInstanceInfo,
) pcommon.Map {
	if len(md.includeResourceAttributes) == 0 {
		filteredAttributes := pcommon.NewMap()
		filteredAttributes.EnsureCapacity(attrs.Len() + collectorInfo.Size())
		attrs.CopyTo(filteredAttributes)
		collectorInfo.Copy(filteredAttributes)
		return filteredAttributes
	}

	filteredAttributes := filterByResolved(attrs, resolved, collectorInfo.Size())
	collectorInfo.Copy(filteredAttributes)
	return filteredAttributes
}

// FilterAttributes builds a pcommon.Map from the resolved attributes
// list. Entries are applied in order so later entries override earlier
// ones for the same key.
func (*MetricDef[K]) FilterAttributes(attrs pcommon.Map, resolved []AttributeKeyValue) pcommon.Map {
	return filterByResolved(attrs, resolved, 0)
}

// filterByResolved creates a pcommon.Map by iterating the resolved
// AttributeKeyValue list in order. Later entries with the same key
// override earlier ones.
func filterByResolved(attrs pcommon.Map, resolved []AttributeKeyValue, extraCapacity int) pcommon.Map {
	dst := pcommon.NewMap()
	dst.EnsureCapacity(len(resolved) + extraCapacity)
	for _, kv := range resolved {
		copyAttribute(kv.Key, kv.DefaultValue, attrs, dst)
	}
	return dst
}

// resolveEntries evaluates OTTL expressions in entries and returns a
// flat, deduplicated, sorted list of AttributeKeyValue. Entries are
// processed in reverse so the last occurrence of each key (highest
// priority) is kept and earlier duplicates are discarded. The result
// is sorted alphabetically by key for deterministic hashing — this is
// safe because dedup guarantees unique keys and pcommon.Map is
// order-independent.
func resolveEntries[K any](ctx context.Context, tCtx K, entries []attributeEntry[K]) ([]AttributeKeyValue, error) {
	seen := make(map[string]struct{}, len(entries))
	resolved := make([]AttributeKeyValue, 0, len(entries))
	for _, entry := range slices.Backward(entries) {
		if entry.Expression != nil {
			keys, err := evalKeysExpression(ctx, tCtx, entry)
			if err != nil {
				return nil, err
			}
			for _, key := range slices.Backward(keys) {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				resolved = append(resolved, AttributeKeyValue{
					Key:          key,
					Optional:     entry.Optional,
					DefaultValue: entry.DefaultValue,
				})
			}
			continue
		}
		if _, ok := seen[entry.Key]; ok {
			continue
		}
		seen[entry.Key] = struct{}{}
		resolved = append(resolved, AttributeKeyValue{
			Key:          entry.Key,
			Optional:     entry.Optional,
			DefaultValue: entry.DefaultValue,
		})
	}
	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].Key < resolved[j].Key
	})
	return resolved, nil
}

// buildStaticAttrs checks if any entries have OTTL expressions and
// builds a pre-sorted list of static-key AttributeKeyValue entries.
// Returns (hasExpr, sortedStaticAttrs).
func buildStaticAttrs[K any](entries []attributeEntry[K]) (bool, []AttributeKeyValue) {
	hasExpr := false
	static := make([]AttributeKeyValue, 0, len(entries))
	for _, entry := range entries {
		if entry.Expression != nil {
			hasExpr = true
			continue
		}
		static = append(static, AttributeKeyValue{
			Key:          entry.Key,
			Optional:     entry.Optional,
			DefaultValue: entry.DefaultValue,
		})
	}
	sort.Slice(static, func(i, j int) bool {
		return static[i].Key < static[j].Key
	})
	return hasExpr, static
}

func parseAttributeEntries[K any](
	cfgs []config.Attribute,
	pc *ottl.ParserCollection[*ottl.ValueExpression[K]],
	contextName string,
) ([]attributeEntry[K], error) {
	var errs []error
	entries := make([]attributeEntry[K], 0, len(cfgs))
	for _, attr := range cfgs {
		val := pcommon.NewValueEmpty()
		if err := val.FromRaw(attr.DefaultValue); err != nil {
			errs = append(errs, err)
			continue
		}
		entry := attributeEntry[K]{
			Optional:     attr.Optional,
			DefaultValue: val,
		}
		if attr.KeysExpression != "" {
			expr, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{attr.KeysExpression}), true)
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

// evalKeysExpression evaluates the OTTL expression and returns the
// resolved attribute keys. Returns nil (no error) if the expression
// evaluates to nil.
func evalKeysExpression[K any](
	ctx context.Context,
	tCtx K,
	entry attributeEntry[K],
) ([]string, error) {
	result, err := entry.Expression.Eval(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate keys_expression: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	keys, err := extractStringSlice(result)
	if err != nil {
		return nil, fmt.Errorf("keys_expression must return a list of strings: %w", err)
	}
	return keys, nil
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
