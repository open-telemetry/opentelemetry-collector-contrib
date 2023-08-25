// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter // import

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

const (
	// The value of "type" key in configuration.
	typeStr = "kinetica"
)

// Config defines configuration for the Kinetica exporter.
type Config struct {
	Host               string              `mapstructure:"host"`
	Schema             string              `mapstructure:"schema"`
	Username           string              `mapstructure:"username"`
	Password           configopaque.String `mapstructure:"password"`
	BypassSslCertCheck bool                `mapstructure:"bypasssslcertcheck"`
	LogConfigFile      string              `mapstructure:"logconfigfile"`
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

	fmt.Println("Password = ", string(cfg.Password))

	return nil
}

var _ component.Config = (*Config)(nil)
