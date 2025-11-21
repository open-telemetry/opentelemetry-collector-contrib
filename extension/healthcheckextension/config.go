// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

// Config is an alias to the shared healthcheck.Config to keep the extensions in lockstep.
// This ensures complete compatibility and eliminates the need for translation layers.
type Config = healthcheck.Config

// Type alias for backward compatibility
type ResponseBodySettings = healthcheck.ResponseBodyConfig
