// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import "go.opentelemetry.io/collector/config/configauth"

type ScraperConfig struct {
	Address string                    `mapstructure:"address"`
	Auth    *configauth.Authentication `mapstructure:"auth"`
}
