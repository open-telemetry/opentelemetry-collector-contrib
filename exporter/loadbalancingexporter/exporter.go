// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

var _ component.TraceExporter = (*exporterImp)(nil)

type exporterImp struct {
	logger *zap.Logger
	config Config
}

// Crete new exporter
func newExporter(params component.ExporterCreateParams, cfg configmodels.Exporter) (*exporterImp, error) {
	oCfg := cfg.(*Config)

	return &exporterImp{
		logger: params.Logger,
		config: *oCfg,
	}, nil
}

func (e *exporterImp) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (e *exporterImp) Shutdown(context.Context) error {
	return nil
}

func (e *exporterImp) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	return nil
}

func (e *exporterImp) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: false}
}
