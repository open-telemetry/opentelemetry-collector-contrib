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

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// CoralogixExporter by Coralogix
type coralogixExporter struct {
	cfg    Config
	logger *zap.Logger
	client coralogixClient
}

// NewCoralogixExporter by Coralogix
func newCoralogixExporter(cfg *Config, params component.ExporterCreateSettings) (*coralogixExporter, error) {
	if cfg.Endpoint == "" || cfg.Endpoint == "https://" || cfg.Endpoint == "http://" {
		return nil, errors.New("coralogix exporter config requires an Endpoint")
	}

	return &coralogixExporter{
		cfg:    *cfg,
		logger: params.Logger,
		client: *newCoralogixClient(cfg, params),
	}, nil
}

func (cx *coralogixExporter) tracesPusher(ctx context.Context, td ptrace.Traces) error {
	return cx.client.newPost(ctx, td)
}
