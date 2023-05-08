// Copyright The OpenTelemetry Authors
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

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"
)

type metricName string

func (mn metricName) Render() (string, error) {
	return formatIdentifier(string(mn), true)
}

func (mn metricName) RenderUnexported() (string, error) {
	return formatIdentifier(string(mn), false)
}

type attributeName string

func (mn attributeName) Render() (string, error) {
	return formatIdentifier(string(mn), true)
}

func (mn attributeName) RenderUnexported() (string, error) {
	return formatIdentifier(string(mn), false)
}

// ValueType defines an attribute value type.
type ValueType struct {
	// ValueType is type of the attribute value.
	ValueType pcommon.ValueType
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (mvt *ValueType) UnmarshalText(text []byte) error {
	switch vtStr := string(text); vtStr {
	case "string":
		mvt.ValueType = pcommon.ValueTypeStr
	case "int":
		mvt.ValueType = pcommon.ValueTypeInt
	case "double":
		mvt.ValueType = pcommon.ValueTypeDouble
	case "bool":
		mvt.ValueType = pcommon.ValueTypeBool
	case "bytes":
		mvt.ValueType = pcommon.ValueTypeBytes
	case "slice":
		mvt.ValueType = pcommon.ValueTypeSlice
	case "map":
		mvt.ValueType = pcommon.ValueTypeMap
	default:
		return fmt.Errorf("invalid type: %q", vtStr)
	}
	return nil
}

// String returns capitalized name of the ValueType.
func (mvt ValueType) String() string {
	return strings.Title(strings.ToLower(mvt.ValueType.String())) // nolint SA1019
}

// Primitive returns name of primitive type for the ValueType.
func (mvt ValueType) Primitive() string {
	switch mvt.ValueType {
	case pcommon.ValueTypeStr:
		return "string"
	case pcommon.ValueTypeInt:
		return "int64"
	case pcommon.ValueTypeDouble:
		return "float64"
	case pcommon.ValueTypeBool:
		return "bool"
	case pcommon.ValueTypeBytes:
		return "[]byte"
	case pcommon.ValueTypeSlice:
		return "[]any"
	case pcommon.ValueTypeMap:
		return "map[string]any"
	default:
		return ""
	}
}

func (mvt ValueType) TestValue() string {
	switch mvt.ValueType {
	case pcommon.ValueTypeEmpty, pcommon.ValueTypeStr:
		return `"attr-val"`
	case pcommon.ValueTypeInt:
		return "1"
	case pcommon.ValueTypeDouble:
		return "1.1"
	case pcommon.ValueTypeBool:
		return "true"
	case pcommon.ValueTypeMap:
		return `map[string]any{"onek": "onev", "twok": "twov"}`
	case pcommon.ValueTypeSlice:
		return `[]any{"one", "two"}`
	}
	return ""
}

type metric struct {
	// Enabled defines whether the metric is enabled by default.
	Enabled bool `mapstructure:"enabled"`

	// Warnings that will be shown to user under specified conditions.
	Warnings warnings `mapstructure:"warnings"`

	// Description of the metric.
	Description string `mapstructure:"description"`

	// ExtendedDocumentation of the metric. If specified, this will
	// be appended to the description used in generated documentation.
	ExtendedDocumentation string `mapstructure:"extended_documentation"`

	// Unit of the metric.
	Unit string `mapstructure:"unit"`

	// Sum stores metadata for sum metric type
	Sum *sum `mapstructure:"sum,omitempty"`
	// Gauge stores metadata for gauge metric type
	Gauge *gauge `mapstructure:"gauge,omitempty"`

	// Attributes is the list of attributes that the metric emits.
	Attributes []attributeName `mapstructure:"attributes"`
}

func (m *metric) Unmarshal(parser *confmap.Conf) error {
	if !parser.IsSet("enabled") {
		return errors.New("missing required field: `enabled`")
	}
	if !parser.IsSet("description") {
		return errors.New("missing required field: `description`")
	}
	err := parser.Unmarshal(m, confmap.WithErrorUnused())
	if err != nil {
		return err
	}
	return nil
}

func (m *metric) Validate() error {
	if m.Sum != nil {
		return m.Sum.Validate()
	}
	if m.Gauge != nil {
		return m.Gauge.Validate()
	}
	return nil
}

func (m metric) Data() MetricData {
	if m.Sum != nil {
		return m.Sum
	}
	if m.Gauge != nil {
		return m.Gauge
	}
	return nil
}

type warnings struct {
	// A warning that will be displayed if the metric is enabled in user config.
	IfEnabled string `mapstructure:"if_enabled"`
	// A warning that will be displayed if `enabled` field is not set explicitly in user config.
	IfEnabledNotSet string `mapstructure:"if_enabled_not_set"`
	// A warning that will be displayed if the metrics is configured by user in any way.
	IfConfigured string `mapstructure:"if_configured"`
}

type attribute struct {
	// Description describes the purpose of the attribute.
	Description string `mapstructure:"description"`
	// NameOverride can be used to override the attribute name.
	NameOverride string `mapstructure:"name_override"`
	// Enabled defines whether the attribute is enabled by default.
	Enabled bool `mapstructure:"enabled"`
	// Enum can optionally describe the set of values to which the attribute can belong.
	Enum []string `mapstructure:"enum"`
	// Type is an attribute type.
	Type ValueType `mapstructure:"type"`
}

func (attr *attribute) Unmarshal(parser *confmap.Conf) error {
	if !parser.IsSet("description") {
		return errors.New("missing required field: `description`")
	}
	if !parser.IsSet("type") {
		return errors.New("missing required field: `type`")
	}
	err := parser.Unmarshal(attr, confmap.WithErrorUnused())
	if err != nil {
		return err
	}
	return nil
}

type metadata struct {
	// Type of the component.
	Type string `mapstructure:"type"`
	// Status information for the component.
	Status Status `mapstructure:"status"`
	// SemConvVersion is a version number of OpenTelemetry semantic conventions applied to the scraped metrics.
	SemConvVersion string `mapstructure:"sem_conv_version"`
	// ResourceAttributes that can be emitted by the component.
	ResourceAttributes map[attributeName]attribute `mapstructure:"resource_attributes"`
	// Attributes emitted by one or more metrics.
	Attributes map[attributeName]attribute `mapstructure:"attributes"`
	// Metrics that can be emitted by the component.
	Metrics map[metricName]metric `mapstructure:"metrics"`
}

func (md *metadata) Unmarshal(parser *confmap.Conf) error {
	if !parser.IsSet("name") {
		return errors.New("missing required field: `description`")
	}
	err := parser.Unmarshal(md, confmap.WithErrorUnused())
	if err != nil {
		return err
	}
	return nil
}

func (md *metadata) Validate() error {
	var errs error

	usedAttrs := map[attributeName]bool{}
	for mn, m := range md.Metrics {
		if m.Sum == nil && m.Gauge == nil {
			errs = multierr.Append(errs, fmt.Errorf("metric %v doesn't have a metric type key, "+
				"one of the following has to be specified: sum, gauge", mn))
			continue
		}
		if m.Sum != nil && m.Gauge != nil {
			errs = multierr.Append(errs, fmt.Errorf("metric %v has more than one metric type keys, "+
				"only one of the following has to be specified: sum, gauge", mn))
			continue
		}

		if err := m.Validate(); err != nil {
			errs = multierr.Append(errs, fmt.Errorf(`metric "%v": %w`, mn, err))
			continue
		}

		unknownAttrs := make([]attributeName, 0, len(m.Attributes))
		for _, attr := range m.Attributes {
			if _, ok := md.Attributes[attr]; ok {
				usedAttrs[attr] = true
			} else {
				unknownAttrs = append(unknownAttrs, attr)
			}
		}
		if len(unknownAttrs) > 0 {
			errs = multierr.Append(errs, fmt.Errorf(`metric "%v" refers to undefined attributes: %v`, mn, unknownAttrs))
		}
	}

	unusedAttrs := make([]attributeName, 0, len(md.Attributes))
	for attr := range md.Attributes {
		if !usedAttrs[attr] {
			unusedAttrs = append(unusedAttrs, attr)
		}
	}
	if len(unusedAttrs) > 0 {
		errs = multierr.Append(errs, fmt.Errorf("unused attributes: %v", unusedAttrs))
	}

	return errs
}

type templateContext struct {
	metadata
	// Package name for generated code.
	Package string
}

func loadMetadata(filePath string) (metadata, error) {
	cp, err := fileprovider.New().Retrieve(context.Background(), "file:"+filePath, nil)
	if err != nil {
		return metadata{}, err
	}

	conf, err := cp.AsConf()
	if err != nil {
		return metadata{}, err
	}

	md := metadata{}
	if err := conf.Unmarshal(&md, confmap.WithErrorUnused()); err != nil {
		return md, err
	}

	if err := md.Validate(); err != nil {
		return md, err
	}

	return md, nil
}
