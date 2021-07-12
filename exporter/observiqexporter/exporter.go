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

package observiqexporter

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func newObservIQLogExporter(config *Config, set component.ExporterCreateSettings) (component.LogsExporter, error) {
	if config == nil {
		return nil, errors.New("config must not be nil")
	}

	if err := config.validateConfig(); err != nil {
		return nil, err
	}

	client, err := buildClient(config, set.Logger, set.BuildInfo)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewLogsExporter(
		config,
		set,
		client.sendLogs,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(config.RetrySettings),
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithShutdown(client.stop),
	)

	if err != nil {
		return nil, err
	}

	return exporter, nil
}
