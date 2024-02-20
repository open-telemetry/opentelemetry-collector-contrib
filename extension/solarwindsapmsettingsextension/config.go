// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"
import "time"

type Config struct {
	Endpoint string        `mapstructure:"endpoint"`
	Key      string        `mapstructure:"key"`
	Interval time.Duration `mapstructure:"interval"`
}
