// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[component.NewID(metadata.Type)]
	assert.Equal(t, p0, factory.CreateDefaultConfig())

	for _, tt := range []struct {
		processor string
		config    *Config
	}{
		{
			processor: "disabled-cloud-namespace",
			config: &Config{
				AddCloudNamespace:           false,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "disabled-attribute-translation",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         false,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "disabled-telegraf-attribute-translation",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: false,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "enabled-nesting",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            true,
					Separator:          "!",
					Include:            []string{"blep"},
					Exclude:            []string{"nghu"},
					SquashSingleValues: true,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "aggregate-attributes",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{
					{
						Attribute: "attr1",
						Prefixes:  []string{"pattern1", "pattern2", "pattern3"},
					},
					{
						Attribute: "attr2",
						Prefixes:  []string{"pattern4"},
					},
				},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "enabled-severity-number-attribute",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{true, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "enabled-severity-text-attribute",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{true, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "enabled-span-id-attribute",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{true, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "enabled-trace-id-attribute",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{true, TraceIDAttributeName},
				},
				TranslateDockerMetrics: false,
			},
		},
		{
			processor: "enabled-docker-metrics-translation",
			config: &Config{
				AddCloudNamespace:           true,
				TranslateAttributes:         true,
				TranslateTelegrafAttributes: true,
				NestAttributes: &NestingProcessorConfig{
					Enabled:            false,
					Separator:          ".",
					Include:            []string{},
					Exclude:            []string{},
					SquashSingleValues: false,
				},
				AggregateAttributes: []aggregationPair{},
				LogFieldsAttributes: &logFieldAttributesConfig{
					SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
					SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
					SpanIDAttribute:         &logFieldAttribute{false, SpanIDAttributeName},
					TraceIDAttribute:        &logFieldAttribute{false, TraceIDAttributeName},
				},
				TranslateDockerMetrics: true,
			},
		},
	} {
		assert.Equal(t, cfg.Processors[component.NewIDWithName(metadata.Type, tt.processor)], tt.config)
	}
}
