// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/splunkhecencodingextension"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	conventions "go.opentelemetry.io/collector/semconv/v1.16.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/splunkhecencodingextension/internal/metadata"
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(_ context.Context, _ extension.CreateSettings, config component.Config) (extension.Extension, error) {
	return &splunkhecEncodingExtension{config: config.(*Config)}, nil
}

func createDefaultConfig() component.Config {
	return &Config{
		Splitting: SplittingStrategyLine,
		HecToOtelAttrs: splunk.HecToOtelAttrs{
			Source:     splunk.DefaultSourceLabel,
			SourceType: splunk.DefaultSourceTypeLabel,
			Index:      splunk.DefaultIndexLabel,
			Host:       conventions.AttributeHostName,
		},
		HecFields: OtelToHecFields{
			SeverityText:   splunk.DefaultSeverityTextLabel,
			SeverityNumber: splunk.DefaultSeverityNumberLabel,
		},
	}
}
