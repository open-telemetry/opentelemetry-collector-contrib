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

	// InstanceLevelMetrics enables collection of metrics for all tasks on the instance.
	// This requires the receiver to run as a Managed Daemon Service on ECS Managed Instances,
	// which provides access to the /tasks and /tasks/stats endpoints.
	InstanceLevelMetrics bool `mapstructure:"instance_level_metrics"`

	// prevent unkeyed literal initialization
	_ struct{}
}
