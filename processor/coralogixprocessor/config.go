// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

// TransactionsConfig holds configuration for transactions.
type TransactionsConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	_       struct{} // prevents unkeyed literal initialization
}

type Config struct {
	TransactionsConfig `mapstructure:"transactions"`
	// prevents unkeyed literal initialization
	_ struct{}
}
