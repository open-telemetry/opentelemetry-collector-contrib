// Copyright  The OpenTelemetry Authors
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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"

// deprecateDescription add [DEPRECATED] prefix to description.
func (m *metricMysqlCommands) deprecateDescription() {
	m.data.SetDescription("[DEPRECATED] " + m.data.Description())
}

// DeprecateMetrics runs deprecation action per metric
func (mb *MetricsBuilder) DeprecateMetrics() {
	mb.metricMysqlCommands.deprecateDescription()
}
