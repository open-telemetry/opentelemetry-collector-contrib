// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
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

	// Optional username. Use the specified Username to authenticate the current connection
	// with one of the connections defined in the ACL list when connecting
	// to a Redis 6.0 instance, or greater, that is using the Redis ACL system.
	Username string `mapstructure:"username"`

	// Optional password. Must match the password specified in the
	// requirepass server configuration option, or the user's password when connecting
	// to a Redis 6.0 instance, or greater, that is using the Redis ACL system.
	Password configopaque.String `mapstructure:"password"`

	TLS configtls.TLSClientSetting `mapstructure:"tls,omitempty"`

	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
}
