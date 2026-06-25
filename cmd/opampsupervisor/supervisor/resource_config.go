// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	telemetryconfig "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	xotelconf "go.opentelemetry.io/contrib/otelconf/x"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

var newExperimentalSDK = xotelconf.NewSDK

func buildSupervisorResourceConfig(ctx context.Context, cfg *config.ResourceConfig) (*telemetryconfig.Resource, error) {
	instanceUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	resourceCfg := cfg.Resource
	resourceCfg.Detectors = nil
	resourceCfg.Attributes = make([]telemetryconfig.AttributeNameValue, 0, len(cfg.Attributes)+len(cfg.LegacyAttributes)+2)

	declarativeAttributeNames := make(map[string]struct{}, len(cfg.Attributes))
	for _, attr := range cfg.Attributes {
		declarativeAttributeNames[attr.Name] = struct{}{}
	}

	defaultAttributes := map[string]any{
		string(conventions.ServiceNameKey):       "opamp-supervisor",
		string(conventions.ServiceInstanceIDKey): instanceUUID.String(),
	}

	for name, value := range defaultAttributes {
		if _, ok := cfg.LegacyAttributes[name]; ok {
			continue
		}
		if _, ok := declarativeAttributeNames[name]; ok {
			continue
		}
		resourceCfg.Attributes = append(resourceCfg.Attributes, telemetryconfig.AttributeNameValue{
			Name:  name,
			Value: value,
		})
	}

	for key, value := range cfg.LegacyAttributes {
		if value == nil {
			continue
		}
		resourceCfg.Attributes = append(resourceCfg.Attributes, telemetryconfig.AttributeNameValue{
			Name:  key,
			Value: value,
		})
	}

	resourceCfg.Attributes = append(resourceCfg.Attributes, cfg.Attributes...)

	if resourceCfg.SchemaUrl == nil {
		schemaURL := conventions.SchemaURL
		resourceCfg.SchemaUrl = &schemaURL
	}

	if cfg.DetectionDevelopment != nil {
		detectionSDK, err := newDetectionResourceSDK(ctx, cfg.DetectionDevelopment)
		if err != nil {
			return nil, err
		}
		attrs := detectionSDK.Resource().Attributes()
		detectedAttrs := make([]telemetryconfig.AttributeNameValue, 0, len(attrs))
		for _, attr := range attrs {
			detectedAttrs = append(detectedAttrs, telemetryconfig.AttributeNameValue{
				Name:  string(attr.Key),
				Value: attr.Value.AsInterface(),
			})
		}
		resourceCfg.Attributes = append(detectedAttrs, resourceCfg.Attributes...)
	}

	return &resourceCfg, nil
}

func newDetectionResourceSDK(ctx context.Context, detection *xotelconf.ExperimentalResourceDetection) (xotelconf.SDK, error) {
	return newExperimentalSDK(
		xotelconf.WithContext(ctx),
		xotelconf.WithOpenTelemetryConfiguration(xotelconf.OpenTelemetryConfiguration{
			Resource: &xotelconf.Resource{DetectionDevelopment: detection},
		}),
	)
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
		if err := attrs.PutEmpty(string(kv.Key)).FromRaw(kv.Value.AsInterface()); err != nil {
			return pcommon.Resource{}, err
		}
	}

	return pcommonResource, nil
}
