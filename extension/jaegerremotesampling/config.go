// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
)

var (
	errTooManySources     = errors.New("too many sources specified, has to be either 'file' or 'remote'")
	errNoSources          = errors.New("no sources specified, has to be either 'file' or 'remote'")
	errAtLeastOneProtocol = errors.New("no protocols selected to serve the strategies, use 'grpc', 'http', or both")
)

// Config has the configuration for the extension enabling the health check
// extension, used to report the health status of the service.
type Config struct {
	*confighttp.HTTPServerSettings `mapstructure:"http"`
	*configgrpc.GRPCServerSettings `mapstructure:"grpc"`

	// Source configures the source for the strategies file. One of `remote` or `file` has to be specified.
	Source Source `mapstructure:"source"`
}

type Source struct {
	// Remote defines the remote location for the file
	Remote *configgrpc.GRPCClientSettings `mapstructure:"remote"`

	// File specifies a local file as the strategies source
	File string `mapstructure:"file"`

	// ReloadInterval determines the periodicity to refresh the strategies
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.HTTPServerSettings == nil && cfg.GRPCServerSettings == nil {
		return errAtLeastOneProtocol
	}

	if cfg.Source.File != "" && cfg.Source.Remote != nil {
		return errTooManySources
	}

	if cfg.Source.File == "" && cfg.Source.Remote == nil {
		return errNoSources
	}

	return nil
}
