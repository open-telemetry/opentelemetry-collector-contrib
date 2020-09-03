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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/collection"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:           "nil config fails",
			wantErr:        true,
			wantErrMessage: "nil config",
		},
		{
			name: "bad config fails",
			config: &Config{
				APIURL: "abc",
			},
			wantErr: true,
		},
		{
			name: "successfully create exporter",
			config: &Config{
				AccessToken: "someToken",
				Realm:       "xyz",
				Timeout:     1 * time.Second,
				Headers:     nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSignalFxExporter(tt.config, zap.NewNop())
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMessage != "" {
					require.EqualError(t, err, tt.wantErrMessage)
				}
			} else {
				require.NotNil(t, got)
				require.NoError(t, got.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, got.Shutdown(context.Background()))
			}
		})
	}
}

func TestConsumeMetrics(t *testing.T) {
	smallBatch := pdatautil.MetricsFromMetricsData(
		[]consumerdata.MetricsData{
			{
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
			},
		},
	)
	tests := []struct {
		name                 string
		md                   pdata.Metrics
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
			md:               generateLargeBatch(),
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
				converter: translation.NewMetricsConverter(zap.NewNop(), nil),
			}

			numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), tt.md)
			assert.Equal(t, tt.numDroppedTimeSeries, numDroppedTimeSeries)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestConsumeMetricsWithAccessTokenPassthrough(t *testing.T) {
	fromHeaders := "AccessTokenFromClientHeaders"
	fromLabels := []string{"AccessTokenFromLabel0", "AccessTokenFromLabel1"}

	validMetricsWithToken := func(includeToken bool, token string) pdata.Metrics {
		md := consumerdata.MetricsData{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "test_signalfx"},
			},
			Resource: &resourcepb.Resource{
				Type: "test",
				Labels: map[string]string{
					"com.splunk.signalfx.access_token": token,
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
		return pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{md})
	}

	tests := []struct {
		name                     string
		accessTokenPassthrough   bool
		metrics                  pdata.Metrics
		additionalHeaders        map[string]string
		failHTTP                 bool
		droppedTimeseriesCount   int
		numPushDataCallsPerToken map[string]int
	}{
		{
			name:                   "passthrough access token and included in md",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0]),
			numPushDataCallsPerToken: map[string]int{
				fromLabels[0]: 1,
			},
		},
		{
			name:                   "passthrough access token and not included in md",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(false, fromLabels[0]),
			numPushDataCallsPerToken: map[string]int{
				fromHeaders: 1,
			},
		},
		{
			name:                   "don't passthrough access token and included in md",
			accessTokenPassthrough: false,
			metrics:                validMetricsWithToken(true, fromLabels[0]),
			numPushDataCallsPerToken: map[string]int{
				fromHeaders: 1,
			},
		},
		{
			name:                   "don't passthrough access token and not included in md",
			accessTokenPassthrough: false,
			metrics:                validMetricsWithToken(false, fromLabels[0]),
			numPushDataCallsPerToken: map[string]int{
				fromHeaders: 1,
			},
		},
		{
			name:                   "override user-specified token-like header",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0]),
			additionalHeaders: map[string]string{
				"x-sf-token": "user-specified",
			},
			numPushDataCallsPerToken: map[string]int{
				fromLabels[0]: 1,
			},
		},
		{
			name:                   "use token from header when resource is nil",
			accessTokenPassthrough: true,
			metrics: pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
				{
					Node: &commonpb.Node{
						ServiceInfo: &commonpb.ServiceInfo{Name: "test_signalfx"},
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
				},
			}),
			numPushDataCallsPerToken: map[string]int{
				fromHeaders: 1,
			},
		},
		{
			name:                   "multiple tokens passed through",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0])
				forSecondToken := validMetricsWithToken(true, fromLabels[1])
				forSecondToken.ResourceMetrics().Resize(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(1))

				return forSecondToken
			}(),
			numPushDataCallsPerToken: map[string]int{
				fromLabels[0]: 1,
				fromLabels[1]: 1,
			},
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0])
				forSecondToken := validMetricsWithToken(true, fromLabels[1])
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1])

				forSecondToken.ResourceMetrics().Resize(3)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(1))
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(2))

				return forSecondToken
			}(),
			numPushDataCallsPerToken: map[string]int{
				fromLabels[0]: 1,
				fromLabels[1]: 2,
			},
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token grouped together",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0])
				forSecondToken := validMetricsWithToken(true, fromLabels[1])
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1])

				forSecondToken.ResourceMetrics().Resize(3)
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(1))
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(2))

				return forSecondToken
			}(),
			numPushDataCallsPerToken: map[string]int{
				fromLabels[0]: 1,
				fromLabels[1]: 1,
			},
		},
		{
			name:                   "multiple tokens passed through - one corrupted",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0])
				forSecondToken := validMetricsWithToken(false, fromLabels[1])
				forSecondToken.ResourceMetrics().Resize(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(1))
				return forSecondToken
			}(),
			numPushDataCallsPerToken: map[string]int{
				fromLabels[0]: 1,
				fromHeaders:   1,
			},
		},
		{
			name:                   "multiple tokens passed through - HTTP error cases",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0])
				forSecondToken := validMetricsWithToken(true, fromLabels[1])
				forSecondToken.ResourceMetrics().Resize(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(1))
				return forSecondToken
			}(),
			failHTTP:               true,
			droppedTimeseriesCount: 2,
		},
	}
	for _, tt := range tests {
		receivedTokens := struct {
			sync.Mutex
			tokens     []string
			totalCalls map[string]int
		}{}
		receivedTokens.tokens = []string{}
		receivedTokens.totalCalls = map[string]int{}
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.failHTTP {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				assert.Equal(t, "test", r.Header.Get("test_header_"))
				receivedTokens.Lock()

				token := r.Header.Get("x-sf-token")
				receivedTokens.tokens = append(receivedTokens.tokens, token)
				receivedTokens.totalCalls[token]++

				receivedTokens.Unlock()
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
				converter:              translation.NewMetricsConverter(zap.NewNop(), nil),
			}

			for k, v := range tt.additionalHeaders {
				dpClient.headers[k] = v
			}

			numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), tt.metrics)

			if tt.failHTTP {
				assert.Equal(t, tt.droppedTimeseriesCount, numDroppedTimeSeries)
				assert.Error(t, err)
				return
			}

			assert.Equal(t, 0, numDroppedTimeSeries)
			assert.NoError(t, err)
			require.Equal(t, tt.numPushDataCallsPerToken, receivedTokens.totalCalls)
			for _, rt := range receivedTokens.tokens {
				_, ok := tt.numPushDataCallsPerToken[rt]
				require.True(t, ok)
			}
		})
	}
}

func generateLargeBatch() pdata.Metrics {
	mds := make([]consumerdata.MetricsData, 65000)

	ts := time.Now()
	for i := 0; i < 65000; i++ {
		mds[i] = consumerdata.MetricsData{
			Node: &commonpb.Node{
				ServiceInfo: &commonpb.ServiceInfo{Name: "test_signalfx"},
			},
			Resource: &resourcepb.Resource{Type: "test" + strconv.Itoa(i)},
			Metrics: []*metricspb.Metric{metricstestutil.Gauge(
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
			)},
		}
	}

	return pdatautil.MetricsFromMetricsData(mds)
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

func BenchmarkExporterConsumeData(b *testing.B) {
	batchSize := 1000
	mds := make([]consumerdata.MetricsData, 0, batchSize)
	for i := 0; i < batchSize; i++ {
		mds = append(mds, testMetricsData()...)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	assert.NoError(b, err)

	dpClient := &sfxDPClient{
		ingestURL: serverURL,
		client: &http.Client{
			Timeout: 1 * time.Second,
		},
		logger: zap.NewNop(),
		zippers: sync.Pool{New: func() interface{} {
			return gzip.NewWriter(nil)
		}},
		converter: translation.NewMetricsConverter(zap.NewNop(), nil),
	}
	metrics := pdatautil.MetricsFromMetricsData(mds)
	for i := 0; i < b.N; i++ {
		numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), metrics)
		assert.NoError(b, err)
		assert.Equal(b, 0, numDroppedTimeSeries)
	}
}
