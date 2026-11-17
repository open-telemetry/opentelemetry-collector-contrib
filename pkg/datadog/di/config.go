// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package di // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/di"

// Config enables native debugger delivery for named DI LogRecords.
type Config struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
}
