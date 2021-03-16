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
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter"
)

type factory struct {
	delegate component.ExporterFactory
}

const (
	// The value of "type" key in configuration.
	typeStr = "stackdriver"
)

// NewFactory creates a factory for the stackdriver exporter
func NewFactory() component.ExporterFactory {
	return &factory{delegate: googlecloudexporter.NewFactory()}
}

func (*factory) Type() configmodels.Type {
	return configmodels.Type(typeStr)
}

func (f *factory) CreateDefaultConfig() configmodels.Exporter {
	cfg := f.delegate.CreateDefaultConfig()
	cfg.(*googlecloudexporter.Config).TypeVal = f.Type()
	return cfg
}

func (f *factory) CreateTracesExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.TracesExporter, error) {
	return f.delegate.CreateTracesExporter(ctx, params, cfg)
}

func (f *factory) CreateMetricsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.MetricsExporter, error) {
	return f.delegate.CreateMetricsExporter(ctx, params, cfg)
}

func (f *factory) CreateLogsExporter(
	ctx context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.LogsExporter, error) {
	return f.delegate.CreateLogsExporter(ctx, params, cfg)
}
