// Copyright 2022 Sumo Logic, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sumologicprocessor

import (
	"path/filepath"
	"testing"

<<<<<<< HEAD:processor/sumologicschemaprocessor/config_test.go
=======
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor/internal/metadata"
>>>>>>> 72356b3064 (chore: rename sumologicschemaprocessor to sumologicprocessor):processor/sumologicprocessor/config_test.go
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[component.NewID(typeStr)]
	assert.Equal(t, p0, factory.CreateDefaultConfig())

	p1 := cfg.Processors[component.NewIDWithName(typeStr, "disabled-cloud-namespace")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p2 := cfg.Processors[component.NewIDWithName(typeStr, "disabled-attribute-translation")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p3 := cfg.Processors[component.NewIDWithName(typeStr, "disabled-telegraf-attribute-translation")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p4 := cfg.Processors[component.NewIDWithName(typeStr, "enabled-nesting")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p5 := cfg.Processors[component.NewIDWithName(typeStr, "aggregate-attributes")]

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
					Patterns:  []string{"pattern1", "pattern2", "pattern3"},
				},
				{
					Attribute: "attr2",
					Patterns:  []string{"pattern4"},
				},
			},
			LogFieldsAttributes: &logFieldAttributesConfig{
				SeverityNumberAttribute: &logFieldAttribute{false, SeverityNumberAttributeName},
				SeverityTextAttribute:   &logFieldAttribute{false, SeverityTextAttributeName},
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p6 := cfg.Processors[component.NewIDWithName(typeStr, "enabled-severity-number-attribute")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p7 := cfg.Processors[component.NewIDWithName(typeStr, "enabled-severity-text-attribute")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p8 := cfg.Processors[component.NewIDWithName(typeStr, "enabled-span-id-attribute")]

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
				SpanIdAttribute:         &logFieldAttribute{true, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p9 := cfg.Processors[component.NewIDWithName(typeStr, "enabled-trace-id-attribute")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{true, TraceIdAttributeName},
			},
			TranslateDockerMetrics: false,
		})

	p10 := cfg.Processors[component.NewIDWithName(typeStr, "enabled-docker-metrics-translation")]

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
				SpanIdAttribute:         &logFieldAttribute{false, SpanIdAttributeName},
				TraceIdAttribute:        &logFieldAttribute{false, TraceIdAttributeName},
			},
			TranslateDockerMetrics: true,
		})
}
