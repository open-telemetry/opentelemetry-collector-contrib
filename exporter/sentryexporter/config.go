// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// DefaultAttributeForProject is the default resource attribute used for project routing
	DefaultAttributeForProject = "service.name"
)

// Config defines the configuration for the Sentry exporter.
type Config struct {
	// URL is the base URL for the Sentry organization (e.g. https://sentry.io).
	URL string `mapstructure:"url"`
	// OrgSlug is the target Sentry organization slug.
	OrgSlug string `mapstructure:"org_slug"`
	// AuthToken is the Sentry auth token used for OTLP ingestion and REST APIs.
	AuthToken configopaque.String `mapstructure:"auth_token"`
	// AutoCreateProjects enables automatic project creation when a destination
	// project is missing in Sentry.
	AutoCreateProjects bool `mapstructure:"auto_create_projects"`
	// Routing controls how resource attributes map to Sentry projects.
	Routing RoutingConfig `mapstructure:"routing"`

	// ClientConfig holds HTTP client options for communicating with Sentry.
	confighttp.ClientConfig `mapstructure:"http"`
	// TimeoutConfig sets the exporter timeout.
	TimeoutConfig exporterhelper.TimeoutConfig `mapstructure:",squash"`
	// QueueConfig configures the sending queue.
	QueueConfig configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
}

type RoutingConfig struct {
	// AttributeToProjectMapping is a user defined map to override the default resource attribute to project mapping
	// for the defined key value pairs.
	AttributeToProjectMapping map[string]string `mapstructure:"attribute_to_project_mapping"`
	// ProjectFromAttribute is the resource attribute name used when mapping a resource attribute to a
	// Sentry project (default: service.name).
	ProjectFromAttribute string `mapstructure:"project_from_attribute"`
}

// projectSlugRegexp mirrors Sentry slug validation for allowed characters and length,
// see (https://github.com/getsentry/sentry/blob/22588621ddc4bba5cdc2dd1a2474b69e04581c7e/src/sentry/utils/slug.py#L17-L24).
var (
	projectSlugRegexp     = regexp.MustCompile(`^[a-z0-9_-]{1,50}$`)
	numericOnlySlugRegexp = regexp.MustCompile(`^\d+$`)
)

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

	return validateRoutingConfig(cfg.Routing)
}

func validateRoutingConfig(routing RoutingConfig) error {
	if len(routing.AttributeToProjectMapping) > maxProjects {
		return fmt.Errorf("'attribute_to_project_mapping' must not define more than %d projects", maxProjects)
	}

	for attrValue, projectSlug := range routing.AttributeToProjectMapping {
		if strings.TrimSpace(attrValue) == "" {
			return errors.New("'attribute_to_project_mapping' contains an empty attribute value")
		}

		if trimmed := strings.TrimSpace(projectSlug); trimmed == "" {
			return fmt.Errorf("'attribute_to_project_mapping' has empty project slug for attribute %q", attrValue)
		} else if projectSlug != trimmed {
			return fmt.Errorf("'attribute_to_project_mapping' project slug for attribute %q must not have leading or trailing whitespace", attrValue)
		}

		if !projectSlugRegexp.MatchString(projectSlug) {
			return fmt.Errorf("'attribute_to_project_mapping' project slug %q for attribute %q must match %q", projectSlug, attrValue, projectSlugRegexp.String())
		}

		if numericOnlySlugRegexp.MatchString(projectSlug) {
			return fmt.Errorf("'attribute_to_project_mapping' project slug %q for attribute %q must not be entirely numeric", projectSlug, attrValue)
		}
	}

	return nil
}
