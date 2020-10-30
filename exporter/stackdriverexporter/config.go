// Copyright 2019, OpenTelemetry Authors
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

package stackdriverexporter

import (
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/api/option"
)

// Config defines configuration for Stackdriver exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	ProjectID                     string                   `mapstructure:"project"`
	UserAgent                     string                   `mapstructure:"user_agent"`
	Endpoint                      string                   `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	ResourceMappings               []ResourceMapping        `mapstructure:"resource_mappings"`
	// GetClientOptions returns additional options to be passed
	// to the underlying Google Cloud API client.
	// Must be set programmatically (no support via declarative config).
	// Optional.
	GetClientOptions func() []option.ClientOption

	TraceConfig  TraceConfig  `mapstructure:"trace"`
	MetricConfig MetricConfig `mapstructure:"metric"`
	NumOfWorkers int          `mapstructure:"number_of_workers"`
}

type TraceConfig struct {
	BundleDelayThreshold time.Duration `mapstructure:"bundle_delay_threshold"`
	BundleCountThreshold int           `mapstructure:"bundle_count_threshold"`
	BundleByteThreshold  int           `mapstructure:"bundle_byte_threshold"`
	BundleByteLimit      int           `mapstructure:"bundle_byte_limit"`
	BufferMaxBytes       int           `mapstructure:"buffer_max_bytes"`
}

type MetricConfig struct {
	Prefix                     string `mapstructure:"prefix"`
	SkipCreateMetricDescriptor bool   `mapstructure:"skip_create_descriptor"`
}

// ResourceMapping defines mapping of resources from source (OpenCensus) to target (Stackdriver).
type ResourceMapping struct {
	SourceType string `mapstructure:"source_type"`
	TargetType string `mapstructure:"target_type"`

	LabelMappings []LabelMapping `mapstructure:"label_mappings"`
}

type LabelMapping struct {
	SourceKey string `mapstructure:"source_key"`
	TargetKey string `mapstructure:"target_key"`
	// Optional flag signals whether we can proceed with transformation if a label is missing in the resource.
	// When required label is missing, we fallback to default resource mapping.
	Optional bool `mapstructure:"optional"`
}
