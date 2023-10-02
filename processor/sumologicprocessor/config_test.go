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

	p1 := cfg.Processors[component.NewIDWithName(metadata.Type, "disabled-cloud-namespace")]

	assert.Equal(t, p1,
		&Config{
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
		})

	p2 := cfg.Processors[component.NewIDWithName(metadata.Type, "disabled-attribute-translation")]

	assert.Equal(t, p2,
		&Config{
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
		})

	p3 := cfg.Processors[component.NewIDWithName(metadata.Type, "disabled-telegraf-attribute-translation")]

	assert.Equal(t, p3,
		&Config{
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
		})

	p4 := cfg.Processors[component.NewIDWithName(metadata.Type, "enabled-nesting")]

	assert.Equal(t, p4,
		&Config{
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
		})

	p5 := cfg.Processors[component.NewIDWithName(metadata.Type, "aggregate-attributes")]

	assert.Equal(t, p5,
		&Config{
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
		})

	p6 := cfg.Processors[component.NewIDWithName(metadata.Type, "enabled-severity-number-attribute")]

	assert.Equal(t, p6,
		&Config{
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
		})

	p7 := cfg.Processors[component.NewIDWithName(metadata.Type, "enabled-severity-text-attribute")]

	assert.Equal(t, p7,
		&Config{
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
		})

	p8 := cfg.Processors[component.NewIDWithName(metadata.Type, "enabled-span-id-attribute")]

	assert.Equal(t, p8,
		&Config{
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
		})

	p9 := cfg.Processors[component.NewIDWithName(metadata.Type, "enabled-trace-id-attribute")]

	assert.Equal(t, p9,
		&Config{
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
		})

	p10 := cfg.Processors[component.NewIDWithName(metadata.Type, "enabled-docker-metrics-translation")]

	assert.Equal(t, p10,
		&Config{
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
		})
}
