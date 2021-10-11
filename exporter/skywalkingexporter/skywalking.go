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

package skywalkingexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
)

type swExporter struct {
	cfg *Config
}

func newSwExporter(_ context.Context, cfg *Config) (*swExporter, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("SkyWalking exporter cfg requires an Endpoint")
	}

	oce := &swExporter{
		cfg: cfg,
	}
	return oce, nil
}

func (oce *swExporter) start(ctx context.Context, host component.Host) error {
	return nil
}

func (oce *swExporter) shutdown(context.Context) error {
	return nil
}

func newExporter(ctx context.Context, cfg *Config) (*swExporter, error) {
	oce, err := newSwExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return oce, nil
}

func (oce *swExporter) pushLogs(_ context.Context, td pdata.Logs) error {
	return nil
}
