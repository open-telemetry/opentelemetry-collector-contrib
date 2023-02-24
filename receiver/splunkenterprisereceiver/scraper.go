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

package splunkenterprisereceiver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
)

type scraper struct {
	httpSettings confighttp.HTTPClientSettings
	params       component.TelemetrySettings

	net  *http.Client
	next consumer.Metrics
}

var (
	_ component.Component        = (*scraper)(nil)
	_ receiver.CreateMetricsFunc = newScraper
)

func newScraper(
	ctx context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	next consumer.Metrics,
) (receiver.Metrics, error) {
	conf, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	return &scraper{
		httpSettings: conf.HTTPClientSettings,
		params:       settings.TelemetrySettings,
		next:         next,
	}, nil
}

func (s *scraper) Start(ctx context.Context, host component.Host) error {
	net, err := s.httpSettings.ToClient(host, s.params)
	if err != nil {
		return err
	}
	s.net = net
	return nil
}

func (s *scraper) Shutdown(_ context.Context) error {
	if s.net != nil {
		s.net.CloseIdleConnections()
	}
	return nil
}

func (s *scraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(clock.FromContext(ctx).Now())

	_ = now

	return pmetric.NewMetrics(), nil
}
