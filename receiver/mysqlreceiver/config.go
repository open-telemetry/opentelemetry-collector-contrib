// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

const (
	defaultStatementEventsDigestTextLimit = 120
	defaultStatementEventsLimit           = 250
	defaultStatementEventsTimeLimit       = 24 * time.Hour
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Username                                string              `mapstructure:"username,omitempty"`
	Password                                configopaque.String `mapstructure:"password,omitempty"`
	Database                                string              `mapstructure:"database,omitempty"`
	AllowNativePasswords                    bool                `mapstructure:"allow_native_passwords,omitempty"`
	confignet.NetAddr                       `mapstructure:",squash"`
	TLS                                     configtls.TLSClientSetting    `mapstructure:"tls,omitempty"`
	MetricsBuilderConfig                    metadata.MetricsBuilderConfig `mapstructure:",squash"`
	StatementEvents                         StatementEventsConfig         `mapstructure:"statement_events"`
}

type StatementEventsConfig struct {
	DigestTextLimit int           `mapstructure:"digest_text_limit"`
	Limit           int           `mapstructure:"limit"`
	TimeLimit       time.Duration `mapstructure:"time_limit"`
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	// Change the default to Insecure = true as we don't want to break
	// existing deployments which does not use TLS by default.
	if !componentParser.IsSet("tls") {
		cfg.TLS = configtls.TLSClientSetting{}
		cfg.TLS.Insecure = true
	}

	return componentParser.Unmarshal(cfg, confmap.WithErrorUnused())
}
