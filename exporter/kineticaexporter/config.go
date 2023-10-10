// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kineticaexporter"

import (
	"errors"
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
)

var (
	kineticaLogger *zap.Logger
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

func parseNumber(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err == nil {
		return v
	}
	return fallback
}

var _ component.Config = (*Config)(nil)
