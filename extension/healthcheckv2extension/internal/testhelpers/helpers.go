// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testhelpers // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/testhelpers"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

func ErrPriority(config *common.ComponentHealthConfig) status.ErrorPriority {
	if config != nil && config.IncludeRecoverable && !config.IncludePermanent {
		return status.PriorityRecoverable
	}
	return status.PriorityPermanent
}
