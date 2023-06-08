// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2016 The Prometheus Authors
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

package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	promapi "github.com/prometheus/prometheus/web/api/v1"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

// testTargetRetriever represents a list of targets to scrape.
// It is used to represent targets as part of test cases.
// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api_test.go#L86
type testTargetRetriever struct {
	activeTargets  map[string][]*scrape.Target
	droppedTargets map[string][]*scrape.Target
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api_test.go#L91
type testTargetParams struct {
	Identifier       string
	Labels           labels.Labels
	DiscoveredLabels labels.Labels
	Params           url.Values
	Reports          []*testReport
	Active           bool
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api_test.go#L100
type testReport struct {
	Start    time.Time
	Duration time.Duration
	Error    error
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api_test.go#L106
func newTestTargetRetriever(targetsInfo []*testTargetParams) *testTargetRetriever {
	var activeTargets map[string][]*scrape.Target
	var droppedTargets map[string][]*scrape.Target
	activeTargets = make(map[string][]*scrape.Target)
	droppedTargets = make(map[string][]*scrape.Target)

	for _, t := range targetsInfo {
		nt := scrape.NewTarget(t.Labels, t.DiscoveredLabels, t.Params)

		for _, r := range t.Reports {
			nt.Report(r.Start, r.Duration, r.Error)
		}

		if t.Active {
			activeTargets[t.Identifier] = []*scrape.Target{nt}
		} else {
			droppedTargets[t.Identifier] = []*scrape.Target{nt}
		}
	}

	return &testTargetRetriever{
		activeTargets:  activeTargets,
		droppedTargets: droppedTargets,
	}
}

var scrapeStart = time.Now().Add(-11 * time.Second)

func (t testTargetRetriever) TargetsActive() map[string][]*scrape.Target {
	return t.activeTargets
}

func (t testTargetRetriever) TargetsDropped() map[string][]*scrape.Target {
	return t.droppedTargets
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api_test.go#L913
// with adaptations as required
func setupTestTargetRetriever(t *testing.T) *testTargetRetriever {
	t.Helper()

	targets := []*testTargetParams{
		{
			Identifier: "test",
			Labels: labels.FromMap(map[string]string{
				model.SchemeLabel:         "http",
				model.AddressLabel:        "example.com:8080",
				model.MetricsPathLabel:    "/metrics",
				model.JobLabel:            "test",
				model.ScrapeIntervalLabel: "15s",
				model.ScrapeTimeoutLabel:  "5s",
			}),
			DiscoveredLabels: labels.EmptyLabels(),
			Params:           url.Values{},
			Reports:          []*testReport{{scrapeStart, 70 * time.Millisecond, nil}},
			Active:           true,
		},
		{
			Identifier: "blackbox",
			Labels: labels.FromMap(map[string]string{
				model.SchemeLabel:         "http",
				model.AddressLabel:        "localhost:9115",
				model.MetricsPathLabel:    "/probe",
				model.JobLabel:            "blackbox",
				model.ScrapeIntervalLabel: "20s",
				model.ScrapeTimeoutLabel:  "10s",
			}),
			DiscoveredLabels: labels.EmptyLabels(),
			Params:           url.Values{"target": []string{"example.com"}},
			Reports:          []*testReport{{scrapeStart, 100 * time.Millisecond, errors.New("failed")}},
			Active:           true,
		},
		{
			Identifier: "blackbox",
			Labels:     labels.EmptyLabels(),
			DiscoveredLabels: labels.FromMap(map[string]string{
				model.SchemeLabel:         "http",
				model.AddressLabel:        "http://dropped.example.com:9115",
				model.MetricsPathLabel:    "/probe",
				model.JobLabel:            "blackbox",
				model.ScrapeIntervalLabel: "30s",
				model.ScrapeTimeoutLabel:  "15s",
			}),
			Params: url.Values{},
			Active: false,
		},
	}

	return newTestTargetRetriever(targets)
}

func TestTargetAPI(t *testing.T) {
	ttr := setupTestTargetRetriever(t)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
	}))
	addr := srv.Listener.Addr().String()
	srv.Close()

	cfg := &Config{
		ServerConfig: &confighttp.HTTPServerSettings{Endpoint: addr},
		ExternalURL:  "http://localhost:9115/",
	}
	logger := zap.NewNop()
	api, err := NewAPI(cfg, logger, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), ttr)
	require.NoError(t, err)

	// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api_test.go#L1017
	// with adaptations as required
	type test struct {
		query    url.Values
		response interface{}
	}

	tests := []test{
		{
			response: &promapi.TargetDiscovery{
				ActiveTargets: []*promapi.Target{
					{
						DiscoveredLabels: map[string]string{},
						Labels: map[string]string{
							"job": "blackbox",
						},
						ScrapePool:         "blackbox",
						ScrapeURL:          "http://localhost:9115/probe?target=example.com",
						GlobalURL:          "http://localhost:9115/probe?target=example.com",
						Health:             "down",
						LastError:          "failed",
						LastScrape:         scrapeStart,
						LastScrapeDuration: 0.1,
						ScrapeInterval:     "20s",
						ScrapeTimeout:      "10s",
					},
					{
						DiscoveredLabels: map[string]string{},
						Labels: map[string]string{
							"job": "test",
						},
						ScrapePool:         "test",
						ScrapeURL:          "http://example.com:8080/metrics",
						GlobalURL:          "http://example.com:8080/metrics",
						Health:             "up",
						LastError:          "",
						LastScrape:         scrapeStart,
						LastScrapeDuration: 0.07,
						ScrapeInterval:     "15s",
						ScrapeTimeout:      "5s",
					},
				},
				DroppedTargets: []*promapi.DroppedTarget{
					{
						DiscoveredLabels: map[string]string{
							"__address__":         "http://dropped.example.com:9115",
							"__metrics_path__":    "/probe",
							"__scheme__":          "http",
							"job":                 "blackbox",
							"__scrape_interval__": "30s",
							"__scrape_timeout__":  "15s",
						},
					},
				},
			},
		},
		{
			query: url.Values{
				"state": []string{"any"},
			},
			response: &promapi.TargetDiscovery{
				ActiveTargets: []*promapi.Target{
					{
						DiscoveredLabels: map[string]string{},
						Labels: map[string]string{
							"job": "blackbox",
						},
						ScrapePool:         "blackbox",
						ScrapeURL:          "http://localhost:9115/probe?target=example.com",
						GlobalURL:          "http://localhost:9115/probe?target=example.com",
						Health:             "down",
						LastError:          "failed",
						LastScrape:         scrapeStart,
						LastScrapeDuration: 0.1,
						ScrapeInterval:     "20s",
						ScrapeTimeout:      "10s",
					},
					{
						DiscoveredLabels: map[string]string{},
						Labels: map[string]string{
							"job": "test",
						},
						ScrapePool:         "test",
						ScrapeURL:          "http://example.com:8080/metrics",
						GlobalURL:          "http://example.com:8080/metrics",
						Health:             "up",
						LastError:          "",
						LastScrape:         scrapeStart,
						LastScrapeDuration: 0.07,
						ScrapeInterval:     "15s",
						ScrapeTimeout:      "5s",
					},
				},
				DroppedTargets: []*promapi.DroppedTarget{
					{
						DiscoveredLabels: map[string]string{
							"__address__":         "http://dropped.example.com:9115",
							"__metrics_path__":    "/probe",
							"__scheme__":          "http",
							"job":                 "blackbox",
							"__scrape_interval__": "30s",
							"__scrape_timeout__":  "15s",
						},
					},
				},
			},
		},
		{
			query: url.Values{
				"state": []string{"active"},
			},
			response: &promapi.TargetDiscovery{
				ActiveTargets: []*promapi.Target{
					{
						DiscoveredLabels: map[string]string{},
						Labels: map[string]string{
							"job": "blackbox",
						},
						ScrapePool:         "blackbox",
						ScrapeURL:          "http://localhost:9115/probe?target=example.com",
						GlobalURL:          "http://localhost:9115/probe?target=example.com",
						Health:             "down",
						LastError:          "failed",
						LastScrape:         scrapeStart,
						LastScrapeDuration: 0.1,
						ScrapeInterval:     "20s",
						ScrapeTimeout:      "10s",
					},
					{
						DiscoveredLabels: map[string]string{},
						Labels: map[string]string{
							"job": "test",
						},
						ScrapePool:         "test",
						ScrapeURL:          "http://example.com:8080/metrics",
						GlobalURL:          "http://example.com:8080/metrics",
						Health:             "up",
						LastError:          "",
						LastScrape:         scrapeStart,
						LastScrapeDuration: 0.07,
						ScrapeInterval:     "15s",
						ScrapeTimeout:      "5s",
					},
				},
				DroppedTargets: []*promapi.DroppedTarget{},
			},
		},
		{
			query: url.Values{
				"state": []string{"Dropped"},
			},
			response: &promapi.TargetDiscovery{
				ActiveTargets: []*promapi.Target{},
				DroppedTargets: []*promapi.DroppedTarget{
					{
						DiscoveredLabels: map[string]string{
							"__address__":         "http://dropped.example.com:9115",
							"__metrics_path__":    "/probe",
							"__scheme__":          "http",
							"job":                 "blackbox",
							"__scrape_interval__": "30s",
							"__scrape_timeout__":  "15s",
						},
					},
				},
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("run %d %q", i, test.query.Encode()), func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://example.com?%s", test.query.Encode()), nil)
			require.NoError(t, err)
			req.RemoteAddr = addr

			res := api.targets(req)
			require.NoError(t, err)
			require.Equal(t, test.response, res.Data)
		})
	}
}
