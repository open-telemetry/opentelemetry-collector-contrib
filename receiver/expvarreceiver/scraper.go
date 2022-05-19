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

package expvarreceiver

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type expVarScraper struct {
	cfg        *Config
	set        *component.ReceiverCreateSettings
	httpClient *http.Client
}

func newExpVarScraper(cfg *Config, set component.ReceiverCreateSettings) *expVarScraper {
	return &expVarScraper{
		cfg: cfg,
		set: &set,
	}
}

func (e expVarScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := e.cfg.HTTPClientSettings.ToClient(host.GetExtensions(), e.set.TelemetrySettings)
	if err != nil {
		return err
	}
	e.httpClient = httpClient
	return nil
}

func (e expVarScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	return metrics, nil
}
