// Copyright OpenTelemetry Authors
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

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

// Config defines configuration for Azure Monitor
type config struct {
	// Azure Blob Storage connection key,
	// which can be found in the Azure Blob Storage resource on the Azure Portal. (no default)
	ConnectionString string `mapstructure:"connection_string"`
	// Logs related configurations
	Logs logsConfig `mapstructure:"logs"`
	// Traces related configurations
	Traces tracesConfig `mapstructure:"traces"`
}

type logsConfig struct {
	// Name of the container where the exporter saves blobs with logs. (default = "logs")
	ContainerName string `mapstructure:"container_name"`
}

type tracesConfig struct {
	// Name of the container where the exporter saves blobs with traces. (default = "traces")
	ContainerName string `mapstructure:"container_name"`
}
