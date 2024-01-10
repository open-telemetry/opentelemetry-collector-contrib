// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"errors"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

// Config defines configuration for the Kinetica exporter.
type Config struct {
	Host               string              `mapstructure:"host"`
	Schema             string              `mapstructure:"schema"`
	Username           string              `mapstructure:"username"`
	Password           configopaque.String `mapstructure:"password"`
	BypassSslCertCheck bool                `mapstructure:"bypasssslcertcheck"`
}

// Validate the config
//
//	@receiver cfg
//	@return error
func (cfg *Config) Validate() error {
	kineticaHost, err := url.ParseRequestURI(cfg.Host)
	if err != nil {
		return err
	}
	if kineticaHost.Scheme != "http" && kineticaHost.Scheme != "https" {
		return errors.New("Protocol must be either `http` or `https`")
	}

	return nil
}

var _ component.Config = (*Config)(nil)
