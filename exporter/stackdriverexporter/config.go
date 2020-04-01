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
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

// Config defines configuration for Stackdriver exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	ProjectID                     string                   `mapstructure:"project"`
	Prefix                        string                   `mapstructure:"metric_prefix"`
	Endpoint                      string                   `mapstructure:"endpoint"`
	NumOfWorkers                  int                      `mapstructure:"number_of_workers"`
	SkipCreateMetricDescriptor    bool                     `mapstructure:"skip_create_metric_descriptor"`
	// Only has effect if Endpoint is not ""
	UseInsecure      bool              `mapstructure:"use_insecure"`
	ResourceMappings []ResourceMapping `mapstructure:"resource_mappings"`
}

// ResourceMapping defines mapping of resources from source (OpenCensus) to target (Stackdriver).
type ResourceMapping struct {
	SourceResourceType string `mapstructure:"source_resource_type"`
	TargetResourceType string `mapstructure:"target_resource_type"`

	LabelMappings []LabelMapping `mapstructure:"label_mappings"`
}

type LabelMapping struct {
	SourceLabelKey string `mapstructure:"source_label_key"`
	TargetLabelKey string `mapstructure:"target_label_key"`
}
