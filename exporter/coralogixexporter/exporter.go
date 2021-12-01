// Copyright 2021, OpenTelemetry Authors
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

package coralogixexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// CoralogixExporter by Coralogix
type CoralogixExporter struct {
	cfg    Config
	logger *zap.Logger
	client CoralogixClient
}

// NewCoralogixExporter by Coralogix
func NewCoralogixExporter(cfg *Config, params component.ExporterCreateSettings) *CoralogixExporter {
	return &CoralogixExporter{
		cfg:    *cfg,
		logger: params.Logger,
		client: *NewCoralogixClient(cfg, params.Logger),
	}
}

func (cx *CoralogixExporter) tracesPusher(_ context.Context, td pdata.Traces) error {
	cx.client.newPost(td)
	return nil
}

// func (cx *CoralogixExporter) start(_ context.Context, host component.Host) (err error) {
// 	return nil
// }
