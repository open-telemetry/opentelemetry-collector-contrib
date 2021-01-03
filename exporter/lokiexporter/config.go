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

package lokiexporter

import (
	"fmt"
	"net/url"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Loki exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// Labels defines how labels should be applied to log streams sent to Loki.
	Labels LabelsConfig `mapstructure:"labels"`
}

func (c *Config) validate() error {
	if _, err := url.Parse(c.Endpoint); c.Endpoint == "" || err != nil {
		return fmt.Errorf("\"endpoint\" must be a valid URL")
	}

	if err := c.Labels.validate(); err != nil {
		return err
	}

	return nil
}

// LabelsConfig defines the labels-related configuration
type LabelsConfig struct {
	// Default is the map of labels to add to every log stream.
	Default map[string]string `mapstructure:"default"`

	// AttributesForLabels is the attributes that are allowed to be added as labels on a log stream.
	AttributesForLabels []string `mapstructure:"attributes_for_labels"`
}

func (c *LabelsConfig) validate() error {
	if len(c.AttributesForLabels) == 0 {
		return fmt.Errorf("\"labels.attributes_for_labels\" must have a least one label")
	}
	return nil
}

// getAttributesToLabelNames creates a lookup of allowed attributes to valid Loki label names.
func (c *LabelsConfig) getAttributesToLabelNames() *map[string]model.LabelName {
	attributesToLabelNames := map[string]model.LabelName{}

	for _, attr := range c.AttributesForLabels {
		_, ok := attributesToLabelNames[attr]
		if !ok {
			attributesToLabelNames[attr] = convertToLabel(attr)
		}
	}

	return &attributesToLabelNames
}

// convertToLabel will convert a string into a valid Prometheus label. Loki labels can only contain ASCII letters,
// digits, and underscores. They cannot start with a digit.
//
// This is faster than an inverse regex of github.com/prometheus/common/model.LabelNameRE and similar in concept to
// github.com/prometheus/common/model.LabelName.IsValid(), but with additional logic for preventing doubling
// underscores.
func convertToLabel(inLabel string) model.LabelName {
	if len(inLabel) == 0 {
		return ""
	}
	outLabel := []byte(inLabel)
	for i, c := range outLabel {
		// If current character is valid and not an underscore, continue on to the next character.
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9' && i > 0) {
			continue
		}

		// If the previous character is either an underscore or a null byte (meaning we've removed the character
		// previously, since the character before it is an underscore), we replace the current character with a
		// null byte, so that underscores are not doubled up (e.g. "_%_example_label" where % is an invalid character).
		if i-1 >= 0 && (outLabel[i-1] == '_' || outLabel[i-1] == 0) {
			outLabel[i] = 0
			continue
		}

		// Worth noting that outLabel[i] may already be an underscore, but it would add extra CPU cycles to check.
		outLabel[i] = '_'
	}
	return model.LabelName(outLabel)
}
