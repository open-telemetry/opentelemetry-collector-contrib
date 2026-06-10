// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"maps"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	telemetryconfig "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	xotelconf "go.opentelemetry.io/contrib/otelconf/x"
	otelsdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func buildSupervisorResourceConfig(ctx context.Context, cfg *config.ResourceConfig) (*telemetryconfig.Resource, error) {
	instanceUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	resourceCfg := cfg.Resource
	resourceCfg.Detectors = nil
	resourceCfg.Attributes = make([]telemetryconfig.AttributeNameValue, 0, len(cfg.Attributes)+len(cfg.LegacyAttributes)+8)

	mergedAttributes := map[string]any{
		string(conventions.ServiceNameKey):       "opamp-supervisor",
		string(conventions.ServiceInstanceIDKey): instanceUUID.String(),
	}

	detectedAttributes, err := detectResourceAttributes(ctx, cfg.DetectionDevelopment)
	if err != nil {
		return nil, err
	}
	maps.Copy(mergedAttributes, detectedAttributes)

	maps.Copy(mergedAttributes, envResourceAttributes())

	for key, value := range cfg.LegacyAttributes {
		if value == nil {
			delete(mergedAttributes, key)
			continue
		}
		mergedAttributes[key] = value
	}

	for _, attr := range cfg.Attributes {
		mergedAttributes[attr.Name] = attr.Value
	}

	for key, value := range mergedAttributes {
		resourceCfg.Attributes = append(resourceCfg.Attributes, telemetryconfig.AttributeNameValue{
			Name:  key,
			Value: value,
		})
	}

	if resourceCfg.SchemaUrl == nil {
		schemaURL := conventions.SchemaURL
		resourceCfg.SchemaUrl = &schemaURL
	}

	return &resourceCfg, nil
}

func detectResourceAttributes(ctx context.Context, cfg *xotelconf.ExperimentalResourceDetection) (map[string]any, error) {
	if cfg == nil {
		return nil, nil
	}

	opts := make([]otelsdkresource.Option, 0, len(cfg.Detectors))
	for _, detector := range cfg.Detectors {
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

	resource, err := otelsdkresource.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	attrs := map[string]any{}
	iter := resource.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		key := string(kv.Key)
		if !includeDetectedAttribute(key, cfg.Attributes) {
			continue
		}
		attrs[key] = kv.Value.AsInterface()
	}

	return attrs, nil
}

func includeDetectedAttribute(key string, filters *xotelconf.IncludeExclude) bool {
	if filters == nil {
		return true
	}
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
		matched, err := filepathMatch(pattern, key)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func filepathMatch(pattern, value string) (bool, error) {
	return path.Match(pattern, value)
}

func envResourceAttributes() map[string]any {
	attrs := map[string]any{}

	raw, ok := os.LookupEnv("OTEL_RESOURCE_ATTRIBUTES")
	if ok {
		for pair := range strings.SplitSeq(raw, ",") {
			key, value, found := strings.Cut(pair, "=")
			if !found {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			attrs[key] = strings.TrimSpace(value)
		}
	}

	if value, ok := os.LookupEnv("OTEL_SERVICE_NAME"); ok {
		attrs[string(conventions.ServiceNameKey)] = value
	}

	return attrs
}

func resourceConfigToPcommon(ctx context.Context, resourceCfg *telemetryconfig.Resource) (pcommon.Resource, error) {
	sdk, err := telemetryconfig.NewSDK(
		telemetryconfig.WithContext(ctx),
		telemetryconfig.WithOpenTelemetryConfiguration(
			telemetryconfig.OpenTelemetryConfiguration{
				Resource: resourceCfg,
			},
		),
	)
	if err != nil {
		return pcommon.Resource{}, err
	}

	sdkResource := sdk.Resource()
	sdkIterator := sdkResource.Iter()

	pcommonResource := pcommon.NewResource()
	attrs := pcommonResource.Attributes()
	attrs.EnsureCapacity(sdkIterator.Len())

	for sdkIterator.Next() {
		kv := sdkIterator.Attribute()
		if err := attrs.PutEmpty(string(kv.Key)).FromRaw(normalizePcommonValue(kv.Value.AsInterface())); err != nil {
			return pcommon.Resource{}, err
		}
	}

	return pcommonResource, nil
}

func normalizePcommonValue(v any) any {
	switch value := v.(type) {
	case []string:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	case []bool:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	case []int:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = int64(item)
		}
		return out
	case []int64:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	case []float64:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	default:
		return v
	}
}
