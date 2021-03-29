// Copyright  OpenTelemetry Authors
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

package ecsobserver

// CommonExporterConfig should be embedded into filter config.
// They set labels like job, metrics_path etc. that can override prometheus default.
type CommonExporterConfig struct {
	JobName      string `mapstructure:"job_name" yaml:"job_name"`
	MetricsPath  string `mapstructure:"metrics_path" yaml:"metrics_path"`
	MetricsPorts []int  `mapstructure:"metrics_ports" yaml:"metrics_ports"`
}
