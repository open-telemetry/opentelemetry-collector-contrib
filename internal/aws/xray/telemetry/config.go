// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import "go.opentelemetry.io/collector/component"

type Config struct {
	// Enabled determines whether any telemetry should be recorded.
	Enabled bool `mapstructure:"enabled"`
	// IncludeMetadata determines whether metadata (instance ID, hostname, resourceARN)
	// should be included in the telemetry.
	IncludeMetadata bool `mapstructure:"include_metadata"`
	// Contributors can be used to explicitly define which X-Ray components are contributing to the telemetry.
	// If omitted, only X-Ray components with the same component.ID as the setup component will have access.
	Contributors []component.ID `mapstructure:"contributors,omitempty"`
	// Hostname can be used to explicitly define the hostname associated with the telemetry.
	Hostname string `mapstructure:"hostname,omitempty"`
	// InstanceID can be used to explicitly define the instance ID associated with the telemetry.
	InstanceID string `mapstructure:"instance_id,omitempty"`
	// ResourceARN can be used to explicitly define the resource ARN associated with the telemetry.
	ResourceARN string `mapstructure:"resource_arn,omitempty"`
}
