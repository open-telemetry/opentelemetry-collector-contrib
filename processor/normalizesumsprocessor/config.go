// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package normalizesumsprocessor

import "go.opentelemetry.io/collector/config"

// Config defines configuration for Resource processor.
type Config struct {
	*config.ProcessorSettings `mapstructure:"-"`

	// Transforms describes the metrics that will be transformed
	Transforms []SumMetrics `mapstructure:"transforms"`
}

// Metric defines the transformation applied to the specific metric
type SumMetrics struct {
	// MetricName is used to select the metric to operate on.
	// REQUIRED
	MetricName string `mapstructure:"metric_name"`

	// NewName specifies the name of the new metric after transforming.
	NewName string `mapstructure:"new_name"`
}
