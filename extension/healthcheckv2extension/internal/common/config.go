// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"

import "time"

type ComponentHealthConfig struct {
	IncludePermanent   bool          `mapstructure:"include_permanent_errors"`
	IncludeRecoverable bool          `mapstructure:"include_recoverable_errors"`
	RecoveryDuration   time.Duration `mapstructure:"recovery_duration"`
}

func (c ComponentHealthConfig) Enabled() bool {
	return c.IncludePermanent || c.IncludeRecoverable
}
