// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"fmt"

	"github.com/prometheus/common/model"
)

// Deprecated: [v0.57.0] will be removed without replacement by v0.61.0. See the Config#Tenant for alternatives.
type Tenant struct {
	// Source defines where to obtain the tenant ID. Possible values: static, context, attribute.
	Source string `mapstruct:"source"`

	// Value will be used by the tenant source provider to lookup the value. For instance,
	// when the source=static, the value is a static value. When the source=context, value
	// should be the context key that holds the tenant information.
	Value string `mapstruct:"value"`
}

// LabelsConfig defines the labels-related configuration
// Deprecated: [v0.57.0] will be removed without replacement by v0.61.0. See the Config#Labels for alternatives.
type LabelsConfig struct {
	// Attributes are the log record attributes that are allowed to be added as labels on a log stream.
	Attributes map[string]string `mapstructure:"attributes"`

	// ResourceAttributes are the resource attributes that are allowed to be added as labels on a log stream.
	ResourceAttributes map[string]string `mapstructure:"resource"`

	// RecordAttributes are the attributes from the record that are allowed to be added as labels on a log stream. Possible keys:
	// traceID, spanID, severity, severityN.
	RecordAttributes map[string]string `mapstructure:"record"`
}

func (c *LabelsConfig) validate() error {
	if len(c.Attributes) == 0 && len(c.ResourceAttributes) == 0 && len(c.RecordAttributes) == 0 {
		return fmt.Errorf("\"labels.attributes\", \"labels.resource\", or \"labels.record\" must be configured with at least one attribute")
	}

	logRecordNameInvalidErr := "the label `%s` in \"labels.attributes\" is not a valid label name. Label names must match " + model.LabelNameRE.String()
	for l, v := range c.Attributes {
		if len(v) > 0 && !model.LabelName(v).IsValid() {
			return fmt.Errorf(logRecordNameInvalidErr, v)
		} else if len(v) == 0 && !model.LabelName(l).IsValid() {
			return fmt.Errorf(logRecordNameInvalidErr, l)
		}
	}

	resourceNameInvalidErr := "the label `%s` in \"labels.resource\" is not a valid label name. Label names must match " + model.LabelNameRE.String()
	for l, v := range c.ResourceAttributes {
		if len(v) > 0 && !model.LabelName(v).IsValid() {
			return fmt.Errorf(resourceNameInvalidErr, v)
		} else if len(v) == 0 && !model.LabelName(l).IsValid() {
			return fmt.Errorf(resourceNameInvalidErr, l)
		}
	}

	possibleRecordAttributes := map[string]bool{
		"traceID":   true,
		"spanID":    true,
		"severity":  true,
		"severityN": true,
	}
	for k := range c.RecordAttributes {
		if _, found := possibleRecordAttributes[k]; !found {
			return fmt.Errorf("record attribute %q not recognized, possible values: traceID, spanID, severity, severityN", k)
		}
	}
	return nil
}

// getAttributes creates a lookup of allowed attributes to valid Loki label names.
func (c *LabelsConfig) getAttributes(labels map[string]string) map[string]model.LabelName {

	attributes := map[string]model.LabelName{}

	for attrName, lblName := range labels {
		if len(lblName) > 0 {
			attributes[attrName] = model.LabelName(lblName)
			continue
		}

		attributes[attrName] = model.LabelName(attrName)
	}

	return attributes
}
