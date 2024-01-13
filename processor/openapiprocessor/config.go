// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

const (
	defaultTimeout        = "5s"
	defaultReloadInterval = "5m"
)

type Config struct {

	// OpenAPI spec to use.
	OpenAPISpecs []string `mapstructure:"openapi_specs"`

	// OpenAPI file path to use.
	OpenAPIFilePaths []string `mapstructure:"openapi_file_paths"`

	// List of OpenAPI endpoints to query.
	OpenAPIEndpoints []string `mapstructure:"openapi_endpoints"`

	// Endpoint directory to query.
	OpenAPIDirectories []string `mapstructure:"openapi_directories"`

	// Timeout for the HTTP request to the endpoint.
	APILoadTimeout string `mapstructure:"api_load_timeout"`

	// Reload interval for the OpenAPI endpoints.
	APIReloadInterval string `mapstructure:"api_reload_interval"`

	// If OpenAPI spec only contains https schemes then enabling this will add http as well and otherway around.
	AllowHTTPAndHTTPS bool `mapstructure:"allow_http_and_https"`

	// List of extensions to store as attributes.
	Extensions []string `mapstructure:"openapi_extensions"`
}
