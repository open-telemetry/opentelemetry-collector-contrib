// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterprocessor

import (
	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/regexp"
)

// Config defines configuration for Resource processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	Metrics MetricFilters `mapstructure:"metrics"`

	Logs LogFilters `mapstructure:"logs"`

	SpanEvents SpanEventFilters `mapstructure:"span_events"`
}

// MetricFilters filters by Metric properties.
type MetricFilters struct {
	// Include match properties describe metrics that should be included in the Collector Service pipeline,
	// all other metrics should be dropped from further processing.
	// If both Include and Exclude are specified, Include filtering occurs first.
	Include *filtermetric.MatchProperties `mapstructure:"include"`

	// Exclude match properties describe metrics that should be excluded from the Collector Service pipeline,
	// all other metrics should be included.
	// If both Include and Exclude are specified, Include filtering occurs first.
	Exclude *filtermetric.MatchProperties `mapstructure:"exclude"`
}

// LogFilters filters by Log properties.
type LogFilters struct {
	// Include match properties describe logs that should be included in the Collector Service pipeline,
	// all other logs should be dropped from further processing.
	// If both Include and Exclude are specified, Include filtering occurs first.
	Include *LogMatchProperties `mapstructure:"include"`
	// Exclude match properties describe logs that should be excluded from the Collector Service pipeline,
	// all other logs should be included.
	// If both Include and Exclude are specified, Include filtering occurs first.
	Exclude *LogMatchProperties `mapstructure:"exclude"`
}

// SpanEventFilters filters by Span Event properties.
type SpanEventFilters struct {
	// Include match properties describe span events that should be included in the Collector Service pipeline,
	// all other span events should be dropped from further processing.
	// If both Include and Exclude are specified, Include filtering occurs first.
	Include *SpanEventMatchProperties `mapstructure:"include"`
	// Exclude match properties describe span events that should be excluded from the Collector Service pipeline,
	// all other span events should be included.
	// If both Include and Exclude are specified, Include filtering occurs first.
	Exclude *SpanEventMatchProperties `mapstructure:"exclude"`
}

// MatchType specifies the strategy for matching against `pdata.Log`s.
type MatchType string

// These are the MatchTypes that users can specify for filtering
// `pdata.Log`s.
const (
	Strict = MatchType(filterset.Strict)
	Regexp = MatchType(filterset.Regexp)
)

// LogMatchProperties specifies the set of properties in a log to match against and the
// type of string pattern matching to use.
type LogMatchProperties struct {
	// LogMatchType specifies the type of matching desired
	LogMatchType MatchType `mapstructure:"match_type"`

	// ResourceAttributes defines a list of possible resource attributes to match logs against.
	// A match occurs if any resource attribute matches all expressions in this given list.
	ResourceAttributes []filterconfig.Attribute `mapstructure:"resource_attributes"`

	// RecordAttributes defines a list of possible record attributes to match logs against.
	// A match occurs if any record attribute matches at least one expression in this given list.
	RecordAttributes []filterconfig.Attribute `mapstructure:"record_attributes"`
}

// SpanEventMatchProperties specifies the set of properties in a span event to match against and the
// type of string pattern matching to use.
type SpanEventMatchProperties struct {
	// SpanEventMatchType specifies the type of matching desired
	SpanEventMatchType MatchType `mapstructure:"match_type"`

	// RegexpConfig specifies options for the Regexp match type
	RegexpConfig *regexp.Config `mapstructure:"regexp"`

	// EventNames specifies the list of string patterns to match event names against.
	// A match occurs if the event name matches at least one string pattern in this list.
	EventNames []string `mapstructure:"event_names"`

	// Attributes defines a list of possible span event attributes to match span events against.
	// A match occurs if any span event attribute matches at least one expression in this given list.
	EventAttributes []filterconfig.Attribute `mapstructure:"event_attributes"`
}

var _ config.Processor = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
