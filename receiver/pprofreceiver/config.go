// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// RemoteConfig configures a scraper that pulls pprof from a remote HTTP endpoint.
type RemoteConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// FileConfig configures a scraper that reads pprof files matching a glob pattern.
type FileConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Include is the glob pattern for pprof files to scrape.
	Include string `mapstructure:"include"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// SelfConfig configures a scraper that profiles the running collector.
type SelfConfig struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Fraction of blocking events that are profiled. A value <= 0 disables
	// profiling. See https://golang.org/pkg/runtime/#SetBlockProfileRate for details.
	BlockProfileFraction int `mapstructure:"block_profile_fraction"`

	// Fraction of mutex contention events that are profiled. A value <= 0
	// disables profiling. See https://golang.org/pkg/runtime/#SetMutexProfileFraction
	// for details.
	MutexProfileFraction int `mapstructure:"mutex_profile_fraction"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ServerConfig configures an HTTP server that accepts pushed pprof data.
type ServerConfig struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines the configuration for the pprof receiver.
type Config struct {
	// Remote configures pulling pprof profiles from a remote HTTP endpoint.
	Remote configoptional.Optional[RemoteConfig] `mapstructure:"remote"`

	// File configures reading pprof profiles from files matching a glob pattern.
	File configoptional.Optional[FileConfig] `mapstructure:"file"`

	// Self configures profiling the running collector.
	Self configoptional.Optional[SelfConfig] `mapstructure:"self"`

	// Server configures an HTTP server that accepts pushed pprof profiles.
	Server configoptional.Optional[ServerConfig] `mapstructure:"server"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if !c.Remote.HasValue() && !c.File.HasValue() && !c.Self.HasValue() && !c.Server.HasValue() {
		return errors.New("at least one of remote, file, self, or server must be configured")
	}
	if c.Remote.HasValue() && c.Remote.Get().Endpoint == "" {
		return errors.New("remote.endpoint must be specified")
	}
	if c.File.HasValue() && c.File.Get().Include == "" {
		return errors.New("file.include must be specified")
	}
	return nil
}
