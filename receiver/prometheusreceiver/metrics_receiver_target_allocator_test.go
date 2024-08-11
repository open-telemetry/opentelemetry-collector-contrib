// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !race

package prometheusreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type MockTargetAllocator struct {
	mu          sync.Mutex // mu protects the fields below.
	endpoints   map[string][]mockTargetAllocatorResponse
	accessIndex map[string]*atomic.Int32
	wg          *sync.WaitGroup
	srv         *httptest.Server
	waitIndex   map[string]int
}

type mockTargetAllocatorResponse struct {
	code int
	data []byte
}

type mockTargetAllocatorResponseRaw struct {
	code int
	data any
}

type hTTPSDResponse struct {
	Targets []string                             `json:"targets"`
	Labels  map[model.LabelName]model.LabelValue `json:"labels"`
}

type expectedMetricRelabelConfigTestResult struct {
	JobName            string
	MetricRelabelRegex relabel.Regexp
}

type expectedTestResultJobMap struct {
	Targets             []string
	Labels              model.LabelSet
	MetricRelabelConfig *expectedMetricRelabelConfigTestResult
}

type expectedTestResult struct {
	empty  bool
	jobMap map[string]expectedTestResultJobMap
}

func (mta *MockTargetAllocator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mta.mu.Lock()
	defer mta.mu.Unlock()

	iptr, ok := mta.accessIndex[req.URL.Path]
	if !ok {
		rw.WriteHeader(404)
		return
	}
	index := int(iptr.Load())
	iptr.Add(1)
	pages := mta.endpoints[req.URL.Path]
	if index >= len(pages) {
		rw.WriteHeader(404)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(pages[index].code)
	_, _ = rw.Write(pages[index].data)

	// release WaitGroup after all endpoints have been hit by Prometheus SD once. After that we will call them manually
	wait := mta.waitIndex[req.URL.Path]
	if index == wait {
		mta.wg.Done()
	}
}

func (mta *MockTargetAllocator) Start() {
	mta.srv.Start()
}

func (mta *MockTargetAllocator) Stop() {
	mta.srv.Close()
}

func transformTAResponseMap(rawResponses map[string][]mockTargetAllocatorResponseRaw) (map[string][]mockTargetAllocatorResponse, map[string]*atomic.Int32, error) {
	responsesMap := make(map[string][]mockTargetAllocatorResponse)
	responsesIndexMap := make(map[string]*atomic.Int32)
	for path, responsesRaw := range rawResponses {
		var responses []mockTargetAllocatorResponse
		for _, responseRaw := range responsesRaw {
			respBodyBytes, err := json.Marshal(responseRaw.data)
			if err != nil {
				return nil, nil, err
			}
			responses = append(responses, mockTargetAllocatorResponse{
				code: responseRaw.code,
				data: respBodyBytes,
			})
		}
		responsesMap[path] = responses

		v := &atomic.Int32{}
		responsesIndexMap[path] = v
	}
	return responsesMap, responsesIndexMap, nil
}

func setupMockTargetAllocator(responses Responses) (*MockTargetAllocator, error) {
	responsesMap, responsesIndexMap, err := transformTAResponseMap(responses.responses)
	if err != nil {
		return nil, err
	}

	mockTA := &MockTargetAllocator{
		endpoints:   responsesMap,
		accessIndex: responsesIndexMap,
		waitIndex:   responses.releaserMap,
		wg:          &sync.WaitGroup{},
	}
	mockTA.srv = httptest.NewUnstartedServer(mockTA)
	mockTA.wg.Add(len(responsesMap))

	return mockTA, nil
}

func labelSetTargetsToList(sets []model.LabelSet) []string {
	result := make([]string, len(sets))
	for i, set := range sets {
		address := set["__address__"]
		result[i] = string(address)
	}
	return result
}

type Responses struct {
	releaserMap map[string]int
	responses   map[string][]mockTargetAllocatorResponseRaw
}

func TestGetScrapeConfigHash(t *testing.T) {
	jobToScrapeConfig1 := map[string]*promconfig.ScrapeConfig{}
	jobToScrapeConfig1["job1"] = &promconfig.ScrapeConfig{
		JobName:         "job1",
		HonorTimestamps: true,
		ScrapeInterval:  model.Duration(30 * time.Second),
		ScrapeTimeout:   model.Duration(30 * time.Second),
		MetricsPath:     "/metrics",
		Scheme:          "http",
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"a"},
				TargetLabel:  "d",
				Action:       relabel.KeepEqual,
			},
		},
	}
	jobToScrapeConfig1["job2"] = &promconfig.ScrapeConfig{
		JobName:         "job2",
		HonorTimestamps: true,
		ScrapeInterval:  model.Duration(30 * time.Second),
		ScrapeTimeout:   model.Duration(30 * time.Second),
		MetricsPath:     "/metrics",
		Scheme:          "http",
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"a"},
				TargetLabel:  "d",
				Action:       relabel.KeepEqual,
			},
		},
	}
	jobToScrapeConfig1["job3"] = &promconfig.ScrapeConfig{
		JobName:         "job3",
		HonorTimestamps: true,
		ScrapeInterval:  model.Duration(30 * time.Second),
		ScrapeTimeout:   model.Duration(30 * time.Second),
		MetricsPath:     "/metrics",
		Scheme:          "http",
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"a"},
				TargetLabel:  "d",
				Action:       relabel.KeepEqual,
			},
		},
	}
	jobToScrapeConfig2 := map[string]*promconfig.ScrapeConfig{}
	jobToScrapeConfig2["job2"] = &promconfig.ScrapeConfig{
		JobName:         "job2",
		HonorTimestamps: true,
		ScrapeInterval:  model.Duration(30 * time.Second),
		ScrapeTimeout:   model.Duration(30 * time.Second),
		MetricsPath:     "/metrics",
		Scheme:          "http",
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"a"},
				TargetLabel:  "d",
				Action:       relabel.KeepEqual,
			},
		},
	}
	jobToScrapeConfig2["job1"] = &promconfig.ScrapeConfig{
		JobName:         "job1",
		HonorTimestamps: true,
		ScrapeInterval:  model.Duration(30 * time.Second),
		ScrapeTimeout:   model.Duration(30 * time.Second),
		MetricsPath:     "/metrics",
		Scheme:          "http",
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"a"},
				TargetLabel:  "d",
				Action:       relabel.KeepEqual,
			},
		},
	}
	jobToScrapeConfig2["job3"] = &promconfig.ScrapeConfig{
		JobName:         "job3",
		HonorTimestamps: true,
		ScrapeInterval:  model.Duration(30 * time.Second),
		ScrapeTimeout:   model.Duration(30 * time.Second),
		MetricsPath:     "/metrics",
		Scheme:          "http",
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"a"},
				TargetLabel:  "d",
				Action:       relabel.KeepEqual,
			},
		},
	}

	hash1, err := getScrapeConfigHash(jobToScrapeConfig1)
	require.NoError(t, err)

	hash2, err := getScrapeConfigHash(jobToScrapeConfig2)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestTargetAllocatorJobRetrieval(t *testing.T) {
	for _, tc := range []struct {
		desc      string
		responses Responses
		cfg       *Config
		want      expectedTestResult
	}{
		{
			desc: "default",
			responses: Responses{
				responses: map[string][]mockTargetAllocatorResponseRaw{
					"/scrape_configs": {
						mockTargetAllocatorResponseRaw{code: 200, data: map[string]map[string]any{
							"job1": {
								"job_name":               "job1",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
							"job2": {
								"job_name":               "job2",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
						}},
					},
					"/jobs/job1/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
					},
					"/jobs/job2/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "alertmanager",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "alertmanager",
								}},
						}},
					},
				},
			},
			cfg: &Config{
				PrometheusConfig: &PromConfig{GlobalConfig: promconfig.DefaultGlobalConfig},
				TargetAllocator: &TargetAllocator{
					Interval:    10 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &PromHTTPSDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{
							BasicAuth: &commonconfig.BasicAuth{
								Username: "user",
								Password: "aPassword",
							},
						},
						RefreshInterval: model.Duration(60 * time.Second),
					},
				},
			},
			want: expectedTestResult{
				empty: false,
				jobMap: map[string]expectedTestResultJobMap{
					"job1": {
						Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter":     "london",
							"__meta_prometheus_job": "node",
						},
					},
					"job2": {Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter":     "london",
							"__meta_prometheus_job": "alertmanager",
						}},
				},
			},
		},
		{
			desc: "update labels and targets",
			responses: Responses{
				responses: map[string][]mockTargetAllocatorResponseRaw{
					"/scrape_configs": {
						mockTargetAllocatorResponseRaw{code: 200, data: map[string]map[string]any{
							"job1": {
								"job_name":               "job1",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
							"job2": {
								"job_name":               "job2",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
						}},
					},
					"/jobs/job1/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
									"test":                  "aTest",
								}},
						}},
					},
					"/jobs/job2/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"10.0.40.3:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "alertmanager",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter": "london",
								}},
						}},
					},
				},
			},
			cfg: &Config{
				PrometheusConfig: &PromConfig{GlobalConfig: promconfig.DefaultGlobalConfig},
				TargetAllocator: &TargetAllocator{
					Interval:    10 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &PromHTTPSDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{},
						RefreshInterval:  model.Duration(60 * time.Second),
					},
				},
			},
			want: expectedTestResult{
				empty: false,
				jobMap: map[string]expectedTestResultJobMap{
					"job1": {
						Targets: []string{"localhost:9090"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter":     "london",
							"__meta_prometheus_job": "node",
							"test":                  "aTest",
						},
					},
					"job2": {Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter": "london",
						}},
				},
			},
		},
		{
			desc: "update job list",
			responses: Responses{
				releaserMap: map[string]int{
					"/scrape_configs": 1,
				},
				responses: map[string][]mockTargetAllocatorResponseRaw{
					"/scrape_configs": {
						mockTargetAllocatorResponseRaw{code: 200, data: map[string]map[string]any{
							"job1": {
								"job_name":               "job1",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
							"job2": {
								"job_name":               "job2",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: map[string]map[string]any{
							"job1": {
								"job_name":               "job1",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
							"job3": {
								"job_name":               "job3",
								"scrape_interval":        "30s",
								"scrape_timeout":         "30s",
								"scrape_protocols":       []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":           "/metrics",
								"scheme":                 "http",
								"relabel_configs":        nil,
								"metric_relabel_configs": nil,
							},
						}},
					},
					"/jobs/job1/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
					},
					"/jobs/job3/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"10.0.40.3:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "alertmanager",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"10.0.40.3:9100"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "alertmanager",
								}},
						}},
					},
				},
			},
			cfg: &Config{
				PrometheusConfig: &PromConfig{GlobalConfig: promconfig.DefaultGlobalConfig},
				TargetAllocator: &TargetAllocator{
					Interval:    10 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &PromHTTPSDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{},
						RefreshInterval:  model.Duration(60 * time.Second),
					},
				},
			},
			want: expectedTestResult{
				empty: false,
				jobMap: map[string]expectedTestResultJobMap{
					"job1": {
						Targets: []string{"localhost:9090"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter":     "london",
							"__meta_prometheus_job": "node",
						},
					},
					"job3": {Targets: []string{"10.0.40.3:9100"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter":     "london",
							"__meta_prometheus_job": "alertmanager",
						}},
				},
			},
		},
		{
			desc: "endpoint is not reachable",
			responses: Responses{
				releaserMap: map[string]int{
					"/scrape_configs": 1, // we are too fast if we ignore the first wait a tick
				},
				responses: map[string][]mockTargetAllocatorResponseRaw{
					"/scrape_configs": {
						mockTargetAllocatorResponseRaw{code: 404, data: map[string]map[string]any{}},
						mockTargetAllocatorResponseRaw{code: 404, data: map[string]map[string]any{}},
					},
				},
			},
			cfg: &Config{
				PrometheusConfig: &PromConfig{GlobalConfig: promconfig.DefaultGlobalConfig},
				TargetAllocator: &TargetAllocator{
					Interval:    50 * time.Millisecond,
					CollectorID: "collector-1",
					HTTPSDConfig: &PromHTTPSDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{},
						RefreshInterval:  model.Duration(60 * time.Second),
					},
				},
			},
			want: expectedTestResult{
				empty:  true,
				jobMap: map[string]expectedTestResultJobMap{},
			},
		},
		{
			desc: "update metric relabel config regex",
			responses: Responses{
				releaserMap: map[string]int{
					"/scrape_configs": 1,
				},
				responses: map[string][]mockTargetAllocatorResponseRaw{
					"/scrape_configs": {
						mockTargetAllocatorResponseRaw{code: 200, data: map[string]map[string]any{
							"job1": {
								"job_name":         "job1",
								"scrape_interval":  "30s",
								"scrape_timeout":   "30s",
								"scrape_protocols": []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":     "/metrics",
								"scheme":           "http",
								"metric_relabel_configs": []map[string]string{
									{
										"separator": ";",
										"regex":     "regex1",
										"action":    "keep",
									},
								},
							},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: map[string]map[string]any{
							"job1": {
								"job_name":         "job1",
								"scrape_interval":  "30s",
								"scrape_timeout":   "30s",
								"scrape_protocols": []string{"OpenMetricsText1.0.0", "OpenMetricsText0.0.1", "PrometheusText0.0.4"},
								"metrics_path":     "/metrics",
								"scheme":           "http",
								"metric_relabel_configs": []map[string]string{
									{
										"separator": ";",
										"regex":     "regex2",
										"action":    "keep",
									},
								},
							},
						}},
					},
					"/jobs/job1/targets": {
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
						mockTargetAllocatorResponseRaw{code: 200, data: []hTTPSDResponse{
							{Targets: []string{"localhost:9090"},
								Labels: map[model.LabelName]model.LabelValue{
									"__meta_datacenter":     "london",
									"__meta_prometheus_job": "node",
								}},
						}},
					},
				},
			},
			cfg: &Config{
				PrometheusConfig: &PromConfig{
					ScrapeConfigs: []*promconfig.ScrapeConfig{
						{
							JobName:         "job1",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(30 * time.Second),
							ScrapeTimeout:   model.Duration(30 * time.Second),
							ScrapeProtocols: promconfig.DefaultScrapeProtocols,
							MetricsPath:     "/metrics",
							Scheme:          "http",
							MetricRelabelConfigs: []*relabel.Config{
								{
									Separator: ";",
									Regex:     relabel.MustNewRegexp("(.*)"),
									Action:    relabel.Keep,
								},
							},
						},
					},
				},
				TargetAllocator: &TargetAllocator{
					Interval:    10 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &PromHTTPSDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{},
						RefreshInterval:  model.Duration(60 * time.Second),
					},
				},
			},
			want: expectedTestResult{
				empty: false,
				jobMap: map[string]expectedTestResultJobMap{
					"job1": {
						Targets: []string{"localhost:9090"},
						Labels: map[model.LabelName]model.LabelValue{
							"__meta_datacenter":     "london",
							"__meta_prometheus_job": "node",
						},
						MetricRelabelConfig: &expectedMetricRelabelConfigTestResult{
							JobName:            "job1",
							MetricRelabelRegex: relabel.MustNewRegexp("regex2"),
						},
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			cms := new(consumertest.MetricsSink)

			allocator, err := setupMockTargetAllocator(tc.responses)
			require.NoError(t, err, "Failed to create allocator", tc.responses)

			allocator.Start()
			defer allocator.Stop()

			tc.cfg.TargetAllocator.Endpoint = allocator.srv.URL // set service URL with the automatic generated one
			receiver := newPrometheusReceiver(receivertest.NewNopSettings(), tc.cfg, cms)

			require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))

			allocator.wg.Wait()

			providers := receiver.discoveryManager.Providers()
			if tc.want.empty {
				// if no base config is supplied and the job retrieval fails then no configuration should be found
				require.Len(t, providers, 0)
				return
			}

			require.NotNil(t, providers)

			for _, provider := range providers {
				require.IsType(t, &promHTTP.Discovery{}, provider.Discoverer())
				httpDiscovery := provider.Discoverer().(*promHTTP.Discovery)
				refresh, err := httpDiscovery.Refresh(ctx)
				require.NoError(t, err)

				// are http configs applied?
				sdConfig := provider.Config().(*promHTTP.SDConfig)
				require.Equal(t, tc.cfg.TargetAllocator.HTTPSDConfig.HTTPClientConfig, sdConfig.HTTPClientConfig)

				for _, group := range refresh {
					found := false
					for job, s := range tc.want.jobMap {
						// find correct job to compare to.
						if !strings.Contains(group.Source, job) {
							continue
						}
						// compare targets
						require.Equal(t, s.Targets, labelSetTargetsToList(group.Targets))

						// compare labels and add __meta_url as this label gets automatically added by the SD.
						// which is identical to the source url
						s.Labels["__meta_url"] = model.LabelValue(sdConfig.URL)
						require.Equal(t, s.Labels, group.Labels)
						if s.MetricRelabelConfig != nil {
							for _, sc := range receiver.cfg.PrometheusConfig.ScrapeConfigs {
								if sc.JobName == s.MetricRelabelConfig.JobName {
									for _, mc := range sc.MetricRelabelConfigs {
										require.Equal(t, s.MetricRelabelConfig.MetricRelabelRegex, mc.Regex)
									}
								}
							}
						}
						found = true
					}
					require.True(t, found, "Returned job is not defined in expected values", group)
				}
			}
		})
	}
}
