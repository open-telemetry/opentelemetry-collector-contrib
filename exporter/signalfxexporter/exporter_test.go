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

package signalfxexporter

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

func TestNew(t *testing.T) {
	got, err := New(nil, zap.NewNop())
	assert.EqualError(t, err, "nil config")
	assert.Nil(t, got)

	config := &Config{
		AccessToken: "someToken",
		Realm:       "xyz",
		Timeout:     1 * time.Second,
		Headers:     nil,
	}
	got, err = New(config, zap.NewNop())
	assert.NoError(t, err)
	require.NotNil(t, got)

	// This is expected to fail.
	err = got.ConsumeMetricsData(context.Background(), consumerdata.MetricsData{})
	assert.Error(t, err)
}

func TestConsumeMetricsData(t *testing.T) {
	smallBatch := &consumerdata.MetricsData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test_signalfx"},
		},
		Resource: &resourcepb.Resource{Type: "test"},
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge(
				"test_gauge",
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					metricstestutil.Double(time.Now(), 123))),
		},
	}
	tests := []struct {
		name                 string
		md                   *consumerdata.MetricsData
		reqTestFunc          func(t *testing.T, r *http.Request)
		httpResponseCode     int
		numDroppedTimeSeries int
		wantErr              bool
	}{
		{
			name:             "happy_path",
			md:               smallBatch,
			reqTestFunc:      nil,
			httpResponseCode: http.StatusAccepted,
		},
		{
			name:                 "response_forbidden",
			md:                   smallBatch,
			reqTestFunc:          nil,
			httpResponseCode:     http.StatusForbidden,
			numDroppedTimeSeries: 1,
			wantErr:              true,
		},
		{
			name:             "large_batch",
			md:               generateLargeBatch(t),
			reqTestFunc:      nil,
			httpResponseCode: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "test", r.Header.Get("test_header_"))
				if tt.reqTestFunc != nil {
					tt.reqTestFunc(t, r)
				}
				w.WriteHeader(tt.httpResponseCode)
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			dpClient := &sfxDPClient{
				ingestURL: serverURL,
				headers:   map[string]string{"test_header_": "test"},
				client: &http.Client{
					Timeout: 1 * time.Second,
				},
				logger: zap.NewNop(),
				zippers: sync.Pool{New: func() interface{} {
					return gzip.NewWriter(nil)
				}},
			}

			numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), *tt.md)
			assert.Equal(t, tt.numDroppedTimeSeries, numDroppedTimeSeries)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestConsumeMetricsDataWithAccessTokenPassthrough(t *testing.T) {
	fromHeaders := "AccessTokenFromClientHeaders"
	fromLabels := "AccessTokenFromLabel"

	newMetricData := func(includeToken bool) consumerdata.MetricsData {
		md := consumerdata.MetricsData{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "test_signalfx"},
			},
			Resource: &resourcepb.Resource{
				Type: "test",
				Labels: map[string]string{
					"com.splunk.signalfx.access_token": fromLabels,
				},
			},
			Metrics: []*metricspb.Metric{
				metricstestutil.Gauge(
					"test_gauge",
					[]string{"k0", "k1"},
					metricstestutil.Timeseries(
						time.Now(),
						[]string{"v0", "v1"},
						metricstestutil.Double(time.Now(), 123))),
			},
		}
		if !includeToken {
			delete(md.Resource.Labels, "com.splunk.signalfx.access_token")
		}
		return md
	}

	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		includedInMetricData   bool
		expectedToken          string
	}{
		{
			name:                   "passthrough access token and included in md",
			accessTokenPassthrough: true,
			includedInMetricData:   true,
			expectedToken:          fromLabels,
		},
		{
			name:                   "passthrough access token and not included in md",
			accessTokenPassthrough: true,
			includedInMetricData:   false,
			expectedToken:          fromHeaders,
		},
		{
			name:                   "don't passthrough access token and included in md",
			accessTokenPassthrough: false,
			includedInMetricData:   true,
			expectedToken:          fromHeaders,
		},
		{
			name:                   "don't passthrough access token and not included in md",
			accessTokenPassthrough: false,
			includedInMetricData:   false,
			expectedToken:          fromHeaders,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "test", r.Header.Get("test_header_"))
				assert.Equal(t, tt.expectedToken, r.Header.Get("x-sf-token"))
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			dpClient := &sfxDPClient{
				ingestURL: serverURL,
				headers: map[string]string{
					"test_header_": "test",
					"X-Sf-Token":   fromHeaders,
				},
				client: &http.Client{
					Timeout: 1 * time.Second,
				},
				logger: zap.NewNop(),
				zippers: sync.Pool{New: func() interface{} {
					return gzip.NewWriter(nil)
				}},
				accessTokenPassthrough: tt.accessTokenPassthrough,
			}

			numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), newMetricData(tt.includedInMetricData))
			assert.Equal(t, 0, numDroppedTimeSeries)
			assert.NoError(t, err)
		})
	}
}

func generateLargeBatch(t *testing.T) *consumerdata.MetricsData {
	md := &consumerdata.MetricsData{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test_signalfx"},
		},
		Resource: &resourcepb.Resource{Type: "test"},
	}

	ts := time.Now()
	for i := 0; i < 65000; i++ {
		md.Metrics = append(md.Metrics,
			metricstestutil.Gauge(
				"test_"+strconv.Itoa(i),
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					&metricspb.Point{
						Timestamp: metricstestutil.Timestamp(ts),
						Value:     &metricspb.Point_Int64Value{Int64Value: int64(i)},
					},
				),
			),
		)
	}

	return md
}

func TestConsumeKubernetesMetadata(t *testing.T) {
	type args struct {
		metadata []*collection.KubernetesMetadataUpdate
	}
	type fields struct {
		payLoad map[string]interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Test property updates",
			fields{
				map[string]interface{}{
					"customProperties": map[string]interface{}{
						"prop_erty1": "val1",
						"property2":  nil,
						"prop_erty3": "val33",
						"property4":  nil,
					},
					"tags":         nil,
					"tagsToRemove": nil,
				},
			},
			args{
				[]*collection.KubernetesMetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: collection.MetadataDelta{
							MetadataToAdd: map[string]string{
								"prop.erty1": "val1",
							},
							MetadataToRemove: map[string]string{
								"property2": "val2",
							},
							MetadataToUpdate: map[string]string{
								"prop.erty3": "val33",
								"property4":  "",
							},
						},
					},
				},
			},
		},
		{
			"Test tag updates",
			fields{
				map[string]interface{}{
					"customProperties": map[string]interface{}{},
					"tags": []interface{}{
						"tag_1",
					},
					"tagsToRemove": []interface{}{
						"tag_2",
					},
				},
			},
			args{
				[]*collection.KubernetesMetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: collection.MetadataDelta{
							MetadataToAdd: map[string]string{
								"tag.1": "",
							},
							MetadataToRemove: map[string]string{
								"tag/2": "",
							},
							MetadataToUpdate: map[string]string{},
						},
					},
				},
			},
		},
		{
			"Test quick successive updates",
			fields{
				map[string]interface{}{
					"customProperties": map[string]interface{}{
						"property1": nil,
						"property2": "val2",
						"property3": nil,
					},
					"tags": []interface{}{
						"tag_2",
					},
					"tagsToRemove": []interface{}{
						"tag_1",
					},
				},
			},
			args{
				[]*collection.KubernetesMetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: collection.MetadataDelta{
							MetadataToAdd: map[string]string{
								"tag.1":     "",
								"property1": "val1",
								"property3": "val3",
							},
							MetadataToRemove: map[string]string{
								"tag/2": "",
							},
							MetadataToUpdate: map[string]string{
								"property2": "val22",
							},
						},
					},
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: collection.MetadataDelta{
							MetadataToAdd: map[string]string{
								"tag/2": "",
							},
							MetadataToRemove: map[string]string{
								"tag.1":     "",
								"property1": "val1",
							},
							MetadataToUpdate: map[string]string{
								"property2": "val2",
								"property3": "val33",
							},
						},
					},
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: collection.MetadataDelta{
							MetadataToAdd: map[string]string{},
							MetadataToRemove: map[string]string{
								"property3": "val33",
							},
							MetadataToUpdate: map[string]string{},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		// Use WaitGroup to ensure the mocked server has encountered
		// a request from the exporter.
		wg := sync.WaitGroup{}
		wg.Add(1)

		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)

				p := map[string]interface{}{
					"customProperties": map[string]*string{},
					"tags":             []string{},
					"tagsToRemove":     []string{},
				}

				err = json.Unmarshal(b, &p)
				require.NoError(t, err)

				require.Equal(t, tt.fields.payLoad, p)
				wg.Done()
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			logger := zap.NewNop()

			dimClient := dimensions.NewDimensionClient(
				context.Background(),
				dimensions.DimensionClientOptions{
					Token:                 "",
					APIURL:                serverURL,
					LogUpdates:            true,
					Logger:                logger,
					SendDelay:             1,
					PropertiesMaxBuffered: 10,
				})
			dimClient.Start()

			se := signalfxExporter{
				logger:                 logger,
				pushKubernetesMetadata: dimClient.PushKubernetesMetadata,
			}

			err = se.ConsumeKubernetesMetadata(tt.args.metadata)

			// Wait for requests to be handled by the mocked server before assertion.
			wg.Wait()

			require.NoError(t, err)
		})
	}
}
