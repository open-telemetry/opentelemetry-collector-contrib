// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	// HTTP defines the HTTP server settings for the Azure Functions invoke endpoints.
	HTTP *confighttp.ServerConfig `mapstructure:"http"`

	// Logs defines configuration for log records received from Azure Functions.
	Logs EncodingConfig `mapstructure:"logs"`

	// Auth is the component.ID of the extension that provides Azure authentication
	Auth component.ID `mapstructure:"auth"`

	// IncludeInvokeMetadata, when true, adds Azure Functions invoke metadata to resource attributes.
	IncludeInvokeMetadata bool `mapstructure:"include_invoke_metadata"`
}

// EncodingConfig holds the encoding extension configuration for a signal type.
type EncodingConfig struct {
	// Encoding identifies the encoding of log records that triggered azure functions.
	Encoding component.ID `mapstructure:"encoding"`
	_        struct{}     // Prevent unkeyed literal initialization
}

// Validate checks if the receiver configuration is valid.
func (cfg *Config) Validate() error {
	var errs []error
	if cfg.HTTP == nil || cfg.HTTP.NetAddr.Endpoint == "" {
		errs = append(errs, errors.New("missing http server settings"))
	}

	if cfg.Logs.Encoding == (component.ID{}) {
		errs = append(errs, errors.New("logs.encoding must be set"))
	}

	return errors.Join(errs...)
}
