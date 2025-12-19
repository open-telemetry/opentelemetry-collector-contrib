// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// DefaultAttributeForProject is the default resource attribute used for project routing
	DefaultAttributeForProject = "service.name"
)

type Config struct {
	URL                string              `mapstructure:"url"`
	OrgSlug            string              `mapstructure:"org_slug"`
	AuthToken          configopaque.String `mapstructure:"auth_token"`
	AutoCreateProjects bool                `mapstructure:"auto_create_projects"`
	Routing            RoutingConfig       `mapstructure:"routing"`

	confighttp.ClientConfig `mapstructure:"http"`
	TimeoutConfig           exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	QueueConfig             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
}

type RoutingConfig struct {
	AttributeToProjectMapping map[string]string `mapstructure:"attribute_to_project_mapping"`
	ProjectFromAttribute      string            `mapstructure:"project_from_attribute"`
}

func (cfg *Config) Validate() error {
	if cfg.URL == "" {
		return errors.New("'url' must be configured")
	}
	if _, err := url.Parse(cfg.URL); err != nil {
		return fmt.Errorf("invalid 'url': %w", err)
	}
	if cfg.OrgSlug == "" {
		return errors.New("'org_slug' is required")
	}
	if cfg.AuthToken == "" {
		return errors.New("'auth_token' is required")
	}
	if cfg.Timeout < 0 {
		return errors.New("'timeout' must be non-negative")
	}

	return nil
}
