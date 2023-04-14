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

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter/internal/metadata"
)

const CfgTypeStr = "dataset"

// NewFactory created new factory with DataSet exporters.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		CfgTypeStr,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.Stability),
		exporter.WithTraces(createTracesExporter, metadata.Stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxDelayMs:      MaxDelayMs,
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}
}
