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
	"net/url"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Loki exporter.
type Config struct {
	config.ExporterSettings       `mapstructure:",squash"`
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// TenantID defines the tenant ID to associate log streams with.
	// Deprecated: [v0.57.0] use the attribute processor to add a `loki.tenant` hint.
	// See this component's documentation for more information on how to specify the hint.
	TenantID *string `mapstructure:"tenant_id"`

	// Labels defines how labels should be applied to log streams sent to Loki.
	// Deprecated: [v0.57.0] use the attribute processor to add a `loki.attribute.labels` hint.
	// See this component's documentation for more information on how to specify the hint.
	Labels *LabelsConfig `mapstructure:"labels"`

	// Allows you to choose the entry format in the exporter.
	// Deprecated: [v0.57.0] Only the JSON format will be supported in the future. If you rely on the
	// "body" format and can't change to JSON, let us know before v0.59.0 by opening a GitHub issue
	// and we'll work with you to find a solution.
	Format *string `mapstructure:"format"`

	// Tenant defines how to obtain the tenant ID
	// Deprecated: [v0.57.0] use the attribute processor to add a `loki.tenant` hint.
	// See this component's documentation for more information on how to specify the hint.
	Tenant *Tenant `mapstructure:"tenant"`
}

func (c *Config) Validate() error {
	if _, err := url.Parse(c.Endpoint); c.Endpoint == "" || err != nil {
		return fmt.Errorf("\"endpoint\" must be a valid URL")
	}

	// further validation is needed only if we are in legacy mode
	if !c.isLegacy() {
		return nil
	}

	if c.Tenant != nil {
		if c.Tenant.Source != "attributes" && c.Tenant.Source != "context" && c.Tenant.Source != "static" {
			return fmt.Errorf("invalid tenant source, must be one of 'attributes', 'context', 'static', but is %s", c.Tenant.Source)
		}

		if c.TenantID != nil && *c.TenantID != "" {
			return fmt.Errorf("both tenant_id and tenant were specified, use only 'tenant' instead")
		}
	}

	if c.Labels != nil {
		return c.Labels.validate()
	}

	return nil
}

func (c *Config) isLegacy() bool {
	if c.Format != nil && *c.Format == "body" {
		return true
	}

	if c.Labels != nil {
		return true
	}

	if c.Tenant != nil {
		return true
	}

	if c.TenantID != nil {
		return true
	}

	return false
}
