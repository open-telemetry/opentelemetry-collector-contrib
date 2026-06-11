// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

// TransactionsConfig holds configuration for transactions.
type TransactionsConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	_       struct{} // prevents unkeyed literal initialization
}

// CriticalPathConfig holds configuration for critical path processing.
type CriticalPathConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	_       struct{} // prevents unkeyed literal initialization
}

type Config struct {
	TransactionsConfig `mapstructure:"transactions"`
	CriticalPathConfig `mapstructure:"critical_path"`
	// prevents unkeyed literal initialization
	_ struct{}
}
