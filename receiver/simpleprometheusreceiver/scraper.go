// Copyright 2020, OpenTelemetry Authors
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

package simpleprometheusreceiver

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
)

type prometheusScraper struct {
	logger *zap.Logger
	config *Config
	url    string
	client *http.Client
}

func (s *prometheusScraper) scrape(ctx context.Context) (pdata.ResourceMetricsSlice, error) {
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	rms := pdata.NewResourceMetricsSlice()
	resp, err := s.client.Get(s.url)
	if err != nil {
		return rms, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := ioutil.ReadAll(resp.Body)
		return rms, fmt.Errorf("prometheus receiver at %s returned status %d: %s", s.url, resp.StatusCode, string(body))
	}

	decoder := expfmt.NewDecoder(resp.Body, expfmt.ResponseFormat(resp.Header))

	for {
		var mf dto.MetricFamily
		err := decoder.Decode(&mf)

		if err == io.EOF {
			break
		} else if err != nil {
			return rms, err
		}

		convertMetricFamily(&mf, rms, ts, s.config)
	}

	return rms, nil
}

func newPrometheusScraper(logger *zap.Logger, cfg *Config) (scraperhelper.ResourceMetricsScraper, error) {
	client, err := cfg.HTTPClientSettings.ToClient()
	if err != nil {
		return nil, err
	}

	url := cfg.Endpoint
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	if !strings.HasPrefix(cfg.MetricsPath, "/") {
		url += "/"
	}
	url += cfg.MetricsPath

	ms := &prometheusScraper{
		logger: logger,
		config: cfg,
		url:    url,
		client: client,
	}
	return scraperhelper.NewResourceMetricsScraper(cfg.Name(), ms.scrape), nil
}
