// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdnotifyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sdnotifyextension"

import (
	"go.opentelemetry.io/collector/confmap"
)

var _ confmap.Validator = (*Config)(nil)

// Config controls how the sd_notify extension talks to systemd.
type Config struct{}

// Validate is called by the collector before Start.
func (Config) Validate() error {
	return nil
}
