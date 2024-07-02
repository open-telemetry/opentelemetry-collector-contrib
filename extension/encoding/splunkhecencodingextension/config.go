// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/splunkhecencodingextension"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"go.opentelemetry.io/collector/component"
)

var _ component.ConfigValidator = (*Config)(nil)

type SplittingStrategy string

const (
	SplittingStrategyLine SplittingStrategy = "line"
	SplittingStrategyNone SplittingStrategy = "none"
)

// OtelToHecFields defines the mapping of attributes to HEC fields
type OtelToHecFields struct {
	// SeverityText informs the exporter to map the severity text field to a specific HEC field.
	SeverityText string `mapstructure:"severity_text"`
	// SeverityNumber informs the exporter to map the severity number field to a specific HEC field.
	SeverityNumber string `mapstructure:"severity_number"`
}

type Config struct {
	// Splitting defines the splitting strategy used by the receiver when ingesting raw events. Can be set to "line" or "none". Default is "line".
	Splitting SplittingStrategy `mapstructure:"splitting"`
	// HecToOtelAttrs creates a mapping from HEC metadata to attributes.
	HecToOtelAttrs splunk.HecToOtelAttrs `mapstructure:"hec_metadata_to_otel_attrs"`
	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `mapstructure:"source"`

	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `mapstructure:"sourcetype"`

	// Splunk index, optional name of the Splunk index.
	Index string `mapstructure:"index"`
	// HecFields creates a mapping from attributes to HEC fields.
	HecFields OtelToHecFields `mapstructure:"otel_to_hec_fields"`
}

func (c *Config) Validate() error {

	return nil
}
