// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testhelpers // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/testhelpers"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

func ErrPriority(cfg *config.ComponentHealthConfig) status.ErrorPriority {
	if cfg != nil && cfg.IncludeRecoverable && !cfg.IncludePermanent {
		return status.PriorityRecoverable
	}
	return status.PriorityPermanent
}
