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

package prometheusreceiver

import (
	"context"
	"encoding/json"
	commonconfig "github.com/prometheus/common/config"
	promConfig "github.com/prometheus/prometheus/config"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

type MockTargetAllocator struct {
	mu          sync.Mutex // mu protects the fields below.
	endpoints   map[string][]mockTargetAllocatorResponse
	accessIndex map[string]*int32
	wg          *sync.WaitGroup
	srv         *httptest.Server
}

type mockTargetAllocatorResponse struct {
	code int
	data []byte
}

type mockTargetAllocatorResponseRaw struct {
	code int
	data interface{}
}

type HTTPSDResponse struct {
	Targets []string                             `json:"targets"`
	Labels  map[model.LabelName]model.LabelValue `json:"labels"`
}

type ExpectedTestResultJobMap struct {
	Targets []string
	Labels  model.LabelSet
	empty   bool
}

type ExpectedTestResult struct {
	empty  bool
	jobMap map[string]ExpectedTestResultJobMap
}

func (mta *MockTargetAllocator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mta.mu.Lock()
	defer mta.mu.Unlock()

	iptr, ok := mta.accessIndex[req.URL.Path]
	if !ok {
		rw.WriteHeader(404)
		return
	}
	index := int(*iptr)
	atomic.AddInt32(iptr, 1)
	pages := mta.endpoints[req.URL.Path]
	if index >= len(pages) {
		rw.WriteHeader(404)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(pages[index].code)
	_, _ = rw.Write(pages[index].data)

	// release WaitGroup after all endpoints have been hit by Prometheus SD once. After that we will call them manually
	if index == 0 {
		mta.wg.Done()
	}
}

func (mta *MockTargetAllocator) Start() {
	mta.srv.Start()
}

func (mta *MockTargetAllocator) Stop() {
	mta.srv.Close()
}

func transformTAResponseMap(rawResponses map[string][]mockTargetAllocatorResponseRaw) (map[string][]mockTargetAllocatorResponse, map[string]*int32, error) {
	responsesMap := make(map[string][]mockTargetAllocatorResponse)
	responsesIndexMap := make(map[string]*int32)
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

		v := int32(0)
		responsesIndexMap[path] = &v
	}
	return responsesMap, responsesIndexMap, nil
}

func setupMockTargetAllocator(rawResponses map[string][]mockTargetAllocatorResponseRaw) (*MockTargetAllocator, error) {
	responsesMap, responsesIndexMap, err := transformTAResponseMap(rawResponses)
	if err != nil {
		return nil, err
	}

	mockTA := &MockTargetAllocator{
		endpoints:   responsesMap,
		accessIndex: responsesIndexMap,
		wg:          &sync.WaitGroup{},
	}
	mockTA.srv = httptest.NewUnstartedServer(mockTA)
	mockTA.wg.Add(len(responsesMap))

	return mockTA, nil
}

func labelSetTargetsToList(sets []model.LabelSet) []string {
	var result []string
	for _, set := range sets {
		address := set["__address__"]
		result = append(result, string(address))
	}
	return result
}

func TestTargetAllocatorJobRetrieval(t *testing.T) {
	for _, tc := range []struct {
		desc      string
		responses map[string][]mockTargetAllocatorResponseRaw
		cfg       *Config
		want      ExpectedTestResult
	}{
		{
			desc: "default",
			responses: map[string][]mockTargetAllocatorResponseRaw{
				"/jobs": {
					mockTargetAllocatorResponseRaw{code: 200, data: map[string]LinkJSON{
						"job1": {Link: "/jobs/job1/targets"},
						"job2": {Link: "/jobs/job2/targets"},
					}},
				},
				"/jobs/job1/targets": {
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "node",
							}},
					}},
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "node",
							}},
					}},
				},
				"/jobs/job2/targets": {
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "alertmanager",
							}},
					}},
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "alertmanager",
							}},
					}},
				},
			},
			cfg: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				PrometheusConfig: &promConfig.Config{},
				TargetAllocator: &TargetAllocator{
					Interval:    60 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &promHTTP.SDConfig{
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
			want: ExpectedTestResult{
				empty: false,
				jobMap: map[string]ExpectedTestResultJobMap{
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
			responses: map[string][]mockTargetAllocatorResponseRaw{
				"/jobs": {
					mockTargetAllocatorResponseRaw{code: 200, data: map[string]LinkJSON{
						"job1": {Link: "/jobs/job1/targets"},
						"job2": {Link: "/jobs/job2/targets"},
					}},
				},
				"/jobs/job1/targets": {
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"localhost:9090", "10.0.10.3:9100", "10.0.10.4:9100", "10.0.10.5:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "node",
							}},
					}},
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"localhost:9090"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "node",
								"test":                  "aTest",
							}},
					}},
				},
				"/jobs/job2/targets": {
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"10.0.40.3:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter":     "london",
								"__meta_prometheus_job": "alertmanager",
							}},
					}},
					mockTargetAllocatorResponseRaw{code: 200, data: []HTTPSDResponse{
						{Targets: []string{"10.0.40.2:9100", "10.0.40.3:9100"},
							Labels: map[model.LabelName]model.LabelValue{
								"__meta_datacenter": "london",
							}},
					}},
				},
			},
			cfg: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				PrometheusConfig: &promConfig.Config{},
				TargetAllocator: &TargetAllocator{
					Interval:    60 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &promHTTP.SDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{},
						RefreshInterval:  model.Duration(60 * time.Second),
					},
				},
			},
			want: ExpectedTestResult{
				empty: false,
				jobMap: map[string]ExpectedTestResultJobMap{
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
			desc: "endpoint is not reachable",
			responses: map[string][]mockTargetAllocatorResponseRaw{
				"/jobs": {
					mockTargetAllocatorResponseRaw{code: 404, data: map[string]LinkJSON{}},
				},
			},
			cfg: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				PrometheusConfig: &promConfig.Config{},
				TargetAllocator: &TargetAllocator{
					Interval:    60 * time.Second,
					CollectorID: "collector-1",
					HTTPSDConfig: &promHTTP.SDConfig{
						HTTPClientConfig: commonconfig.HTTPClientConfig{},
						RefreshInterval:  model.Duration(60 * time.Second),
					},
				},
			},
			want: ExpectedTestResult{
				empty:  true,
				jobMap: map[string]ExpectedTestResultJobMap{},
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
			receiver := newPrometheusReceiver(componenttest.NewNopReceiverCreateSettings(), tc.cfg, cms)

			require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))

			allocator.wg.Wait()

			providers := receiver.discoveryManager.Providers()

			if tc.want.empty {
				// if no base config is supplied and the job retrieval fails and therefor no configuration is available
				// PrometheusSD adds a static provider as default

				// we have here a race condition inside the discovery manager. `providers` can be nil or an array of
				// the size of 1
				if providers == nil {
					return
				}
				require.Len(t, providers, 1)
			}

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
						found = true
					}
					require.True(t, found, "Returned job is not defined in expected values", group)
				}
			}
		})
	}
}
