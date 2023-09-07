// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
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
	MetricsBuilderConfig                    metadata.MetricsBuilderConfig `mapstructure:",squash"`
	StatementEvents                         StatementEventsConfig         `mapstructure:"statement_events"`
}

type StatementEventsConfig struct {
	DigestTextLimit int           `mapstructure:"digest_text_limit"`
	Limit           int           `mapstructure:"limit"`
	TimeLimit       time.Duration `mapstructure:"time_limit"`
}
