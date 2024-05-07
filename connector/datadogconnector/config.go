// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the Datadog connector.
type Config struct {
	// Traces defines the Traces specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

// TracesConfig defines the traces specific configuration options
type TracesConfig struct {
	// ignored resources
	// A blocklist of regular expressions can be provided to disable certain traces based on their resource name
	// all entries must be surrounded by double quotes and separated by commas.
	// ignore_resources: ["(GET|POST) /healthcheck"]
	IgnoreResources []string `mapstructure:"ignore_resources"`

	// SpanNameRemappings is the map of datadog span names and preferred name to map to. This can be used to
	// automatically map Datadog Span Operation Names to an updated value. All entries should be key/value pairs.
	// span_name_remappings:
	//   io.opentelemetry.javaagent.spring.client: spring.client
	//   instrumentation:express.server: express
	//   go.opentelemetry.io_contrib_instrumentation_net_http_otelhttp.client: http.client
	SpanNameRemappings map[string]string `mapstructure:"span_name_remappings"`

	// If set to true the OpenTelemetry span name will used in the Datadog resource name.
	// If set to false the resource name will be filled with the instrumentation library name + span kind.
	// The default value is `false`.
	SpanNameAsResourceName bool `mapstructure:"span_name_as_resource_name"`

	// If set to true, enables an additional stats computation check on spans to see they have an eligible `span.kind` (server, consumer, client, producer).
	// If enabled, a span with an eligible `span.kind` will have stats computed. If disabled, only top-level and measured spans will have stats computed.
	// NOTE: For stats computed from OTel traces, only top-level spans are considered when this option is off.
	ComputeStatsBySpanKind bool `mapstructure:"compute_stats_by_span_kind"`

	// If set to true, enables aggregation of peer related tags (e.g., `peer.service`, `db.instance`, etc.) in the datadog connector.
	// If disabled, aggregated trace stats will not include these tags as dimensions on trace metrics.
	// For the best experience with peer tags, Datadog also recommends enabling `compute_stats_by_span_kind`.
	// If you are using an OTel tracer, it's best to have both enabled because client/producer spans with relevant peer tags
	// may not be marked by the datadog connector as top-level spans.
	// If enabling both causes the datadog connector to consume too many resources, try disabling `compute_stats_by_span_kind` first.
	// A high cardinality of peer tags or APM resources can also contribute to higher CPU and memory consumption.
	// You can check for the cardinality of these fields by making trace search queries in the Datadog UI.
	// The default list of peer tags can be found in https://github.com/DataDog/datadog-agent/blob/main/pkg/trace/stats/concentrator.go.
	PeerTagsAggregation bool `mapstructure:"peer_tags_aggregation"`

	// [BETA] Optional list of supplementary peer tags that go beyond the defaults. The Datadog backend validates all tags
	// and will drop ones that are unapproved. The default set of peer tags can be found at
	// https://github.com/DataDog/datadog-agent/blob/505170c4ac8c3cbff1a61cf5f84b28d835c91058/pkg/trace/stats/concentrator.go#L55.
	PeerTags []string `mapstructure:"peer_tags"`

	// TraceBuffer specifies the number of Datadog Agent TracerPayloads to buffer before dropping.
	// The default value is 1000.
	TraceBuffer int `mapstructure:"trace_buffer"`

	// ResourceAttributesAsContainerTags specifies the list of resource attributes to be used as container tags.
	ResourceAttributesAsContainerTags []string `mapstructure:"resource_attributes_as_container_tags"`
}

// Validate the configuration for errors. This is required by component.Config.
func (c *Config) Validate() error {
	if c.Traces.IgnoreResources != nil {
		for _, entry := range c.Traces.IgnoreResources {
			_, err := regexp.Compile(entry)
			if err != nil {
				return fmt.Errorf("%q is not valid resource filter regular expression", entry)
			}
		}
	}

	if c.Traces.SpanNameRemappings != nil {
		for key, value := range c.Traces.SpanNameRemappings {
			if value == "" {
				return fmt.Errorf("%q is not valid value for span name remapping", value)
			}
			if key == "" {
				return fmt.Errorf("%q is not valid key for span name remapping", key)
			}
		}
	}

	if c.Traces.TraceBuffer < 0 {
		return fmt.Errorf("Trace buffer must be non-negative")
	}

	return nil
}
