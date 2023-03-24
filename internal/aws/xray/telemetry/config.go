// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
