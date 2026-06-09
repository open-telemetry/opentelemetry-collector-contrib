// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"path"
	"reflect"
	"slices"
	"sort"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	xotelconf "go.opentelemetry.io/contrib/otelconf/x"
	otelattribute "go.opentelemetry.io/otel/attribute"
	otelsdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func buildSupervisorResourceConfig(ctx context.Context, cfg *config.ResourceConfig) (*otelconf.Resource, pcommon.Resource, error) {
	instanceUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, pcommon.Resource{}, err
	}

	defaultAttributes := map[string]any{
		string(conventions.ServiceNameKey):       "opamp-supervisor",
		string(conventions.ServiceInstanceIDKey): instanceUUID.String(),
	}
	suppressed := suppressedLegacyAttributes(cfg.LegacyAttributes)
	explicitConfigured := configuredAttributes(cfg)

	resource := pcommon.NewResource()
	attrs := resource.Attributes()

	for name, value := range defaultAttributes {
		if _, ok := suppressed[name]; ok && !explicitConfigured[name] {
			continue
		}
		if putErr := putAttributeValue(attrs, name, value); putErr != nil {
			return nil, pcommon.Resource{}, putErr
		}
	}

	envResource := otelsdkresource.Environment()
	envIterator := envResource.Iter()
	for envIterator.Next() {
		kv := envIterator.Attribute()
		key := string(kv.Key)
		if _, ok := suppressed[key]; ok && !explicitConfigured[key] {
			continue
		}
		if putErr := putAttributeValue(attrs, key, kv.Value.AsInterface()); putErr != nil {
			return nil, pcommon.Resource{}, putErr
		}
	}

	detectedResource, err := detectExperimentalResource(ctx, cfg.DetectionDevelopment)
	if err != nil {
		return nil, pcommon.Resource{}, err
	}
	if detectedResource != nil {
		detectedIterator := detectedResource.Iter()
		for detectedIterator.Next() {
			kv := detectedIterator.Attribute()
			key := string(kv.Key)
			if !includeDetectedAttribute(key, cfg.DetectionDevelopment) {
				continue
			}
			if _, ok := suppressed[key]; ok && !explicitConfigured[key] {
				continue
			}
			if _, exists := attrs.Get(key); exists && !explicitConfigured[key] {
				continue
			}
			if err := putAttributeValue(attrs, key, kv.Value.AsInterface()); err != nil {
				return nil, pcommon.Resource{}, err
			}
		}
	}

	for key, value := range cfg.LegacyAttributes {
		if value == nil {
			continue
		}
		if err := putAttributeValue(attrs, key, value); err != nil {
			return nil, pcommon.Resource{}, err
		}
	}

	for _, attr := range cfg.Attributes {
		if err := putAttributeValue(attrs, attr.Name, attr.Value); err != nil {
			return nil, pcommon.Resource{}, err
		}
	}

	fixedResource := &otelconf.Resource{
		Attributes: resourceAttributes(resource),
	}
	if cfg.SchemaUrl != nil {
		schemaURL := *cfg.SchemaUrl
		fixedResource.SchemaUrl = &schemaURL
	} else {
		schemaURL := conventions.SchemaURL
		fixedResource.SchemaUrl = &schemaURL
	}

	return fixedResource, resource, nil
}

func configuredAttributes(cfg *config.ResourceConfig) map[string]bool {
	explicit := make(map[string]bool, len(cfg.LegacyAttributes)+len(cfg.Attributes))
	for key, value := range cfg.LegacyAttributes {
		if value != nil {
			explicit[key] = true
		}
	}
	for _, attr := range cfg.Attributes {
		explicit[attr.Name] = true
	}
	return explicit
}

func suppressedLegacyAttributes(attrs map[string]any) map[string]struct{} {
	suppressed := make(map[string]struct{})
	for key, value := range attrs {
		if value == nil {
			suppressed[key] = struct{}{}
		}
	}
	return suppressed
}

func includeDetectedAttribute(key string, detection *xotelconf.ExperimentalResourceDetection) bool {
	if detection == nil || detection.Attributes == nil {
		return true
	}
	filters := detection.Attributes
	if len(filters.Included) > 0 && !matchesAnyPattern(key, filters.Included) {
		return false
	}
	if len(filters.Excluded) > 0 && matchesAnyPattern(key, filters.Excluded) {
		return false
	}
	return true
}

func matchesAnyPattern(key string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := path.Match(pattern, key)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func detectExperimentalResource(ctx context.Context, detection *xotelconf.ExperimentalResourceDetection) (*otelsdkresource.Resource, error) {
	if detection == nil || len(detection.Detectors) == 0 {
		return nil, nil
	}

	opts := make([]otelsdkresource.Option, 0, len(detection.Detectors)*2)
	for _, detector := range detection.Detectors {
		if detector.Container != nil {
			opts = append(opts, otelsdkresource.WithContainer())
		}
		if detector.Host != nil {
			opts = append(opts, otelsdkresource.WithHost(), otelsdkresource.WithOS())
		}
		if detector.Process != nil {
			opts = append(opts, otelsdkresource.WithProcess())
		}
		if detector.Service != nil {
			opts = append(opts, otelsdkresource.WithService())
		}
	}
	if len(opts) == 0 {
		return nil, nil
	}

	return otelsdkresource.New(ctx, opts...)
}

func putAttributeValue(attrs pcommon.Map, key string, value any) error {
	attrs.Remove(key)
	return attrs.PutEmpty(key).FromRaw(normalizeAttributeValue(value))
}

func resourceAttributes(resource pcommon.Resource) []otelconf.AttributeNameValue {
	values := make([]otelconf.AttributeNameValue, 0, resource.Attributes().Len())
	keys := make([]string, 0, resource.Attributes().Len())
	resource.Attributes().Range(func(key string, _ pcommon.Value) bool {
		keys = append(keys, key)
		return true
	})
	sort.Strings(keys)

	for _, key := range keys {
		value, _ := resource.Attributes().Get(key)
		values = append(values, otelconf.AttributeNameValue{
			Name:  key,
			Value: normalizeAttributeValue(value.AsRaw()),
		})
	}

	return values
}

func normalizeAttributeValue(value any) any {
	switch v := value.(type) {
	case []otelattribute.KeyValue:
		out := make(map[string]any, len(v))
		for _, item := range v {
			out[string(item.Key)] = normalizeAttributeValue(item.Value.AsInterface())
		}
		return out
	case []any:
		out := slices.Clone(v)
		for i := range out {
			out[i] = normalizeAttributeValue(out[i])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeAttributeValue(item)
		}
		return out
	default:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			return value
		}
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			out := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				out[i] = normalizeAttributeValue(rv.Index(i).Interface())
			}
			return out
		}
		return value
	}
}
