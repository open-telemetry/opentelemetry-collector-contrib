// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"

import (
	"time"
)

// Config defines configuration for aws ecs container metrics receiver.
type Config struct {
	// CollectionInterval is the interval at which metrics should be collected
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// IgnoreMissingEndpoint allows receiver to run without failing when ECS metadata endpoint is not available
	IgnoreMissingEndpoint bool `mapstructure:"ignore_missing_endpoint"`

	// prevent unkeyed literal initialization
	_ struct{}
}
