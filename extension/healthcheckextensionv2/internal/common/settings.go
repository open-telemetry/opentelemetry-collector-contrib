// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/common"

import "time"

type ComponentHealthSettings struct {
	IncludePermanent   bool          `mapstructure:"include_permanent_errors"`
	IncludeRecoverable bool          `mapstructure:"include_recoverable_errors"`
	RecoveryDuration   time.Duration `mapstructure:"recovery_duration"`
}

func (c ComponentHealthSettings) Enabled() bool {
	return c.IncludePermanent || c.IncludeRecoverable
}
