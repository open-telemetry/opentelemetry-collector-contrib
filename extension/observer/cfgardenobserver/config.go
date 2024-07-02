// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgardenobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver"

import (
	"time"
)

// Config defines configuration for cf garden observer.
type Config struct {
	// The URL of the cf garden api.  Default is "unix:///var/vcap/data/garden/garden.sock"
	Endpoint string `mapstructure:"endpoint"`

	// RefreshInterval determines how frequency at which the observer
	// needs to poll for collecting information about new processes.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}
