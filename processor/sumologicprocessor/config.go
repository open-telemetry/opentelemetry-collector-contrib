// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	AddCloudNamespace           bool                      `mapstructure:"add_cloud_namespace"`
	TranslateAttributes         bool                      `mapstructure:"translate_attributes"`
	TranslateTelegrafAttributes bool                      `mapstructure:"translate_telegraf_attributes"`
	NestAttributes              *NestingProcessorConfig   `mapstructure:"nest_attributes"`
	AggregateAttributes         []aggregationPair         `mapstructure:"aggregate_attributes"`
	LogFieldsAttributes         *logFieldAttributesConfig `mapstructure:"field_attributes"`
	TranslateDockerMetrics      bool                      `mapstructure:"translate_docker_metrics"`
}

type aggregationPair struct {
	Attribute string   `mapstructure:"attribute"`
	Patterns  []string `mapstructure:"prefixes"`
}

const (
	defaultAddCloudNamespace           = true
	defaultTranslateAttributes         = true
	defaultTranslateTelegrafAttributes = true
	defaultTranlateDockerMetrics       = false

	// Nesting processor default config
	defaultNestingEnabled            = false
	defaultNestingSeparator          = "."
	defaultNestingSquashSingleValues = false

	defaultAddSeverityNumberAttribute = false
	defaultAddSeverityTextAttribute   = false
	defaultAddSpanIDAttribute         = false
	defaultAddTraceIDAttribute        = false
)

var (
	defaultAggregateAttributes = []aggregationPair{}
)

// Ensure the Config struct satisfies the config.Processor interface.
var (
	_                     component.Config = (*Config)(nil)
	defaultNestingInclude                  = []string{}
	defaultNestingExclude                  = []string{}
)

func createDefaultConfig() component.Config {
	return &Config{
		AddCloudNamespace:           defaultAddCloudNamespace,
		TranslateAttributes:         defaultTranslateAttributes,
		TranslateTelegrafAttributes: defaultTranslateTelegrafAttributes,
		NestAttributes: &NestingProcessorConfig{
			Separator:          defaultNestingSeparator,
			Enabled:            defaultNestingEnabled,
			Include:            defaultNestingInclude,
			Exclude:            defaultNestingExclude,
			SquashSingleValues: defaultNestingSquashSingleValues,
		},
		AggregateAttributes: defaultAggregateAttributes,
		LogFieldsAttributes: &logFieldAttributesConfig{
			SeverityNumberAttribute: &logFieldAttribute{defaultAddSeverityNumberAttribute, SeverityNumberAttributeName},
			SeverityTextAttribute:   &logFieldAttribute{defaultAddSeverityTextAttribute, SeverityTextAttributeName},
			SpanIDAttribute:         &logFieldAttribute{defaultAddSpanIDAttribute, SpanIDAttributeName},
			TraceIDAttribute:        &logFieldAttribute{defaultAddTraceIDAttribute, TraceIDAttributeName},
		},
		TranslateDockerMetrics: defaultTranlateDockerMetrics,
	}
}

// Validate config
func (cfg *Config) Validate() error {
	return nil
}
