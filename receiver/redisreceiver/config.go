// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	// TODO: Use one of the configs from core.
	// The target endpoint.
	confignet.NetAddr `mapstructure:",squash"`

	// TODO allow users to add additional resource key value pairs?

	// Optional password. Must match the password specified in the
	// requirepass server configuration option.
	Password string `mapstructure:"password"`

	TLS configtls.TLSClientSetting `mapstructure:"tls,omitempty"`

	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
}
