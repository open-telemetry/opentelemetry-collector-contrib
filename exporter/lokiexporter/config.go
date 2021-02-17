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

	// TenantID defines the tenant ID to associate log streams with.
	TenantID string `mapstructure:"tenant_id"`

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
	// Attributes are the attributes that are allowed to be added as labels on a log stream.
	Attributes map[string]string `mapstructure:"attributes"`
}

func (c *LabelsConfig) validate() error {
	if len(c.Attributes) == 0 {
		return fmt.Errorf("\"labels.attributes\" must be configured with at least one attribute")
	}

	labelNameInvalidErr := "the label `%s` in \"labels.attributes\" is not a valid label name. Label names must match " + model.LabelNameRE.String()
	for l, v := range c.Attributes {
		if len(v) > 0 && !model.LabelName(v).IsValid() {
			return fmt.Errorf(labelNameInvalidErr, v)
		} else if len(v) == 0 && !model.LabelName(l).IsValid() {
			return fmt.Errorf(labelNameInvalidErr, l)
		}
	}

	return nil
}

// getAttributes creates a lookup of allowed attributes to valid Loki label names.
func (c *LabelsConfig) getAttributes() map[string]model.LabelName {
	attributes := map[string]model.LabelName{}

	for attrName, lblName := range c.Attributes {
		if len(lblName) > 0 {
			attributes[attrName] = model.LabelName(lblName)
			continue
		}

		attributes[attrName] = model.LabelName(attrName)
	}

	return attributes
}
