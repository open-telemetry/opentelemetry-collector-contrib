// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/redisstorageextension"

import (
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

// Config defines configuration for the Redis storage extension.
type Config struct {
	Endpoint   string                 `mapstructure:"endpoint"`
	Password   configopaque.String    `mapstructure:"password"`
	DB         int                    `mapstructure:"db"`
	Expiration time.Duration          `mapstructure:"expiration"`
	Prefix     string                 `mapstructure:"prefix"`
	TLS        configtls.ClientConfig `mapstructure:"tls,omitempty"`
}
