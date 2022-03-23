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

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr             = "azureblob"
	logsContainerName   = "logs"
	tracesContainerName = "traces"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Blob Config")
)

// NewFactory returns a factory for Azure Blob exporter.
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(createTracesExporter),
		exporterhelper.WithLogs(createLogsExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings:    config.NewExporterSettings(config.NewComponentID(typeStr)),
		LogsContainerName:   logsContainerName,
		TracesContainerName: tracesContainerName,
	}
}

func createTracesExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	// exporterConfig, ok := cfg.(*Config)

	// if !ok {
	// 	return nil, errUnexpectedConfigurationType
	// }

	// bc, err := NewBlobClient(exporterConfig.ConnectionString, exporterConfig.TracesContainerName, set.Logger)
	// if err != nil {
	// 	set.Logger.Error(err.Error())
	// }

	// return newTracesExporter(exporterConfig, bc, set)

	return exporterhelper.NewTracesExporter(cfg, set, onTraceData)
}

func createLogsExporter(
	ctx context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	// exporterConfig, ok := cfg.(*Config)

	// if !ok {
	// 	return nil, errUnexpectedConfigurationType
	// }

	// bc, err := NewBlobClient(exporterConfig.ConnectionString, exporterConfig.LogsContainerName, set.Logger)
	// if err != nil {
	// 	set.Logger.Error(err.Error())
	// }

	// return newLogsExporter(exporterConfig, bc, set)
	return exporterhelper.NewLogsExporter(config, set, onLogData)
}
