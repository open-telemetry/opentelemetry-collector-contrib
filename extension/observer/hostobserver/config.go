// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver"

import (
	"time"
)

// Config defines configuration for host observer.
type Config struct {
	// RefreshInterval determines how frequency at which the observer
	// needs to poll for collecting information about new processes.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`

	// prevent unkeyed literal initialization
	_ struct{}
}
