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

package signalfxcorrelationexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

// corrExporter is a wrapper struct of the correlation exporter
type corrExporter struct {
	logger *zap.Logger
	config *Config
}

func (se *corrExporter) Shutdown(context.Context) error {
	return nil
}

func newCorrExporter(cfg *Config, params component.ExporterCreateParams) (corrExporter, error) {
	err := cfg.validate()
	if err != nil {
		return corrExporter{}, err
	}

	return corrExporter{
		logger: params.Logger,
		config: cfg,
	}, err
}

func newTraceExporter(cfg *Config, params component.ExporterCreateParams) (component.TraceExporter, error) {
	se, err := newCorrExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraceExporter(
		cfg,
		se.pushTraceData,
		exporterhelper.WithShutdown(se.Shutdown))
}

// pushTraceData processes traces to extract services and environments to associate them to their emitting host/pods.
func (se *corrExporter) pushTraceData(ctx context.Context, td pdata.Traces) (droppedSpansCount int, err error) {
	return 0, nil
}
