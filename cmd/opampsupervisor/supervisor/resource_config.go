// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	telemetryconfig "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func buildSupervisorResourceConfig(cfg *config.ResourceConfig) (*telemetryconfig.Resource, error) {
	instanceUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	resourceCfg := cfg.Resource
	resourceCfg.Detectors = nil
	resourceCfg.Attributes = make([]telemetryconfig.AttributeNameValue, 0, len(cfg.Attributes)+len(cfg.LegacyAttributes)+2)

	defaultAttributes := map[string]any{
		string(conventions.ServiceNameKey):       "opamp-supervisor",
		string(conventions.ServiceInstanceIDKey): instanceUUID.String(),
	}

	for name, value := range defaultAttributes {
		if _, ok := cfg.LegacyAttributes[name]; ok {
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

	return &resourceCfg, nil
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
