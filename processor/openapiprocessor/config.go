// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the OpenAPI processor.
type Config struct {
	// OpenAPIFile is the path to the OpenAPI specification file (YAML or JSON).
	// Can also be an HTTP/HTTPS URL to fetch the spec from a remote server.
	// This is a required field.
	OpenAPIFile string `mapstructure:"openapi_file"`

	// RefreshInterval specifies how often to refresh the OpenAPI spec when using an HTTP URL.
	// If set to 0 or not specified, the spec will only be loaded once at startup.
	// Only applicable when openapi_file is an HTTP/HTTPS URL.
	// Default is 0 (no refresh).
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`

	// URLAttribute specifies the span attribute that contains the URL path to match.
	// Default is "http.url" if not specified. Common values include:
	// - "http.url" (full URL)
	// - "http.target" (path and query string)
	// - "url.path" (semantic conventions v1.21+)
	URLAttribute string `mapstructure:"url_attribute"`

	// PeerService specifies the value to set for the peer.service attribute.
	// If not set, it will be derived from the OpenAPI info.title field.
	// You can also use a template with ${info.title} placeholder.
	PeerService string `mapstructure:"peer_service"`

	// URLTemplateAttribute specifies the attribute name to store the matched URL template.
	// Default is "url.template".
	URLTemplateAttribute string `mapstructure:"url_template_attribute"`

	// PeerServiceAttribute specifies the attribute name to store the peer service.
	// Default is "peer.service".
	PeerServiceAttribute string `mapstructure:"peer_service_attribute"`

	// OverwriteExisting specifies whether to overwrite existing url.template and
	// peer.service attributes if they already exist.
	// Default is false.
	OverwriteExisting bool `mapstructure:"overwrite_existing"`

	// IncludeQueryParams specifies whether to include query parameters in the URL
	// matching. If false, query parameters are stripped before matching.
	// Default is false.
	IncludeQueryParams bool `mapstructure:"include_query_params"`

	// UseServerURLMatching enables matching traces against the servers field
	// in the OpenAPI spec. When enabled, the processor will check if the full
	// URL in the trace starts with any of the server URLs defined in the spec.
	// Only traces that match a server URL will be processed.
	// This is useful for determining which OpenAPI spec/service a trace belongs to.
	// Default is false.
	UseServerURLMatching bool `mapstructure:"use_server_url_matching"`
}

var _ component.Config = (*Config)(nil)

var (
	errMissingOpenAPIFile = errors.New("openapi_file is required")
)

// Validate checks if the processor configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.OpenAPIFile == "" {
		return errMissingOpenAPIFile
	}
	return nil
}
