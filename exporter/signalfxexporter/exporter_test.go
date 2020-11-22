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

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
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
				AccessToken:     "someToken",
				Realm:           "xyz",
				TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
				Headers:         nil,
			},
		},
		{
			name: "create exporter with host metadata syncer",
			config: &Config{
				AccessToken:      "someToken",
				Realm:            "xyz",
				TimeoutSettings:  exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
				Headers:          nil,
				SyncHostMetadata: true,
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
			}
		})
	}
}

func TestConsumeMetrics(t *testing.T) {
	smallBatch := pdata.NewMetrics()
	smallBatch.ResourceMetrics().Resize(1)
	rm := smallBatch.ResourceMetrics().At(0)
	rm.InitEmpty()

	rm.InstrumentationLibraryMetrics().Resize(1)
	ilm := rm.InstrumentationLibraryMetrics().At(0)
	ilm.InitEmpty()
	ilm.Metrics()

	ilm.Metrics().Resize(1)
	m := ilm.Metrics().At(0)

	m.SetName("test_gauge")
	m.SetDataType(pdata.MetricDataTypeDoubleGauge)
	m.DoubleGauge().InitEmpty()

	dp := pdata.NewDoubleDataPoint()
	dp.InitEmpty()
	dp.LabelsMap().InitFromMap(map[string]string{
		"k0": "v0",
		"k1": "v1",
	})
	dp.SetValue(123)

	m.DoubleGauge().DataPoints().Append(dp)

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
			md:               generateLargeDPBatch(t),
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
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					headers:   map[string]string{"test_header_": "test"},
					client: &http.Client{
						Timeout: 1 * time.Second,
					},
					zippers: sync.Pool{New: func() interface{} {
						return gzip.NewWriter(nil)
					}},
				},
				logger:    zap.NewNop(),
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
		out := pdata.NewMetrics()
		out.ResourceMetrics().Resize(1)
		rm := out.ResourceMetrics().At(0)
		rm.InitEmpty()

		if includeToken {
			rm.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
				"com.splunk.signalfx.access_token": pdata.NewAttributeValueString(token),
			})
		}

		rm.InstrumentationLibraryMetrics().Resize(1)
		ilm := rm.InstrumentationLibraryMetrics().At(0)
		ilm.InitEmpty()
		ilm.Metrics()

		ilm.Metrics().Resize(1)
		m := ilm.Metrics().At(0)

		m.SetName("test_gauge")
		m.SetDataType(pdata.MetricDataTypeDoubleGauge)
		m.DoubleGauge().InitEmpty()

		dp := pdata.NewDoubleDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(map[string]string{
			"k0": "v0",
			"k1": "v1",
		})
		dp.SetValue(123)

		m.DoubleGauge().DataPoints().Append(dp)
		return out
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
			metrics: func() pdata.Metrics {
				out := pdata.NewMetrics()
				out.ResourceMetrics().Resize(1)
				rm := out.ResourceMetrics().At(0)
				rm.InitEmpty()

				rm.InstrumentationLibraryMetrics().Resize(1)
				ilm := rm.InstrumentationLibraryMetrics().At(0)
				ilm.InitEmpty()
				ilm.Metrics()

				ilm.Metrics().Resize(1)
				m := ilm.Metrics().At(0)

				m.SetName("test_gauge")
				m.SetDataType(pdata.MetricDataTypeDoubleGauge)
				m.DoubleGauge().InitEmpty()

				dp := pdata.NewDoubleDataPoint()
				dp.InitEmpty()
				dp.LabelsMap().InitFromMap(map[string]string{
					"k0": "v0",
					"k1": "v1",
				})
				dp.SetValue(123)

				m.DoubleGauge().DataPoints().Append(dp)

				return out
			}(),
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
				// We don't do grouping anymore with pdata.Metrics since they
				// are so hard to manipulate.
				fromLabels[1]: 2,
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
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					headers: map[string]string{
						"test_header_": "test",
						"X-Sf-Token":   fromHeaders,
					},
					client: &http.Client{
						Timeout: 1 * time.Second,
					},
					zippers: sync.Pool{New: func() interface{} {
						return gzip.NewWriter(nil)
					}},
				},
				logger:                 zap.NewNop(),
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

func TestNewEventExporter(t *testing.T) {
	got, err := newEventExporter(nil, zap.NewNop())
	assert.EqualError(t, err, "nil config")
	assert.Nil(t, got)

	config := &Config{
		AccessToken:     "someToken",
		IngestURL:       "asdf://:%",
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
		Headers:         nil,
	}

	got, err = newEventExporter(config, zap.NewNop())
	assert.NotNil(t, err)
	assert.Nil(t, got)

	config = &Config{
		AccessToken:     "someToken",
		Realm:           "xyz",
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: 1 * time.Second},
		Headers:         nil,
	}

	got, err = newEventExporter(config, zap.NewNop())
	assert.NoError(t, err)
	require.NotNil(t, got)

	// This is expected to fail.
	rls := makeSampleResourceLogs()
	ld := pdata.NewLogs()
	ld.ResourceLogs().Append(rls)
	_, err = got.pushLogs(context.Background(), ld)
	assert.Error(t, err)
}

func makeSampleResourceLogs() pdata.ResourceLogs {
	logSlice := pdata.NewLogSlice()

	logSlice.Resize(1)
	l := logSlice.At(0)

	l.SetName("shutdown")
	l.SetTimestamp(pdata.TimestampUnixNano(1000))
	attrs := l.Attributes()

	attrs.InitFromMap(map[string]pdata.AttributeValue{
		"k0": pdata.NewAttributeValueString("v0"),
		"k1": pdata.NewAttributeValueString("v1"),
		"k2": pdata.NewAttributeValueString("v2"),
	})

	propMapVal := pdata.NewAttributeValueMap()
	propMap := propMapVal.MapVal()
	propMap.InitFromMap(map[string]pdata.AttributeValue{
		"env":      pdata.NewAttributeValueString("prod"),
		"isActive": pdata.NewAttributeValueBool(true),
		"rack":     pdata.NewAttributeValueInt(5),
		"temp":     pdata.NewAttributeValueDouble(40.5),
	})
	propMap.Sort()
	attrs.Insert("com.splunk.signalfx.event_properties", propMapVal)
	attrs.Insert("com.splunk.signalfx.event_category", pdata.NewAttributeValueInt(int64(sfxpb.EventCategory_USER_DEFINED)))

	l.Attributes().Sort()

	out := pdata.NewResourceLogs()
	out.InitEmpty()
	out.InstrumentationLibraryLogs().Resize(1)
	out.InstrumentationLibraryLogs().At(0).InitEmpty()
	logSlice.MoveAndAppendTo(out.InstrumentationLibraryLogs().At(0).Logs())

	return out
}

func TestConsumeEventData(t *testing.T) {
	tests := []struct {
		name                 string
		resourceLogs         pdata.ResourceLogs
		reqTestFunc          func(t *testing.T, r *http.Request)
		httpResponseCode     int
		numDroppedLogRecords int
		wantErr              bool
	}{
		{
			name:             "happy_path",
			resourceLogs:     makeSampleResourceLogs(),
			reqTestFunc:      nil,
			httpResponseCode: http.StatusAccepted,
		},
		{
			name: "no_event_attribute",
			resourceLogs: func() pdata.ResourceLogs {
				out := makeSampleResourceLogs()
				out.InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().Delete("com.splunk.signalfx.event_category")
				return out
			}(),
			reqTestFunc:          nil,
			numDroppedLogRecords: 1,
			httpResponseCode:     http.StatusAccepted,
		},
		{
			name: "nonconvertible_log_attrs",
			resourceLogs: func() pdata.ResourceLogs {
				out := makeSampleResourceLogs()

				attrs := out.InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes()
				mapAttr := pdata.NewAttributeValueMap()
				attrs.Insert("map", mapAttr)

				propsAttrs, _ := attrs.Get("com.splunk.signalfx.event_properties")
				propsAttrs.MapVal().Insert("map", mapAttr)

				return out
			}(),
			reqTestFunc: nil,
			// The log does go through, just without that prop
			numDroppedLogRecords: 0,
			httpResponseCode:     http.StatusAccepted,
		},
		{
			name:                 "response_forbidden",
			resourceLogs:         makeSampleResourceLogs(),
			reqTestFunc:          nil,
			httpResponseCode:     http.StatusForbidden,
			numDroppedLogRecords: 1,
			wantErr:              true,
		},
		{
			name:             "large_batch",
			resourceLogs:     generateLargeEventBatch(t),
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

			eventClient := &sfxEventClient{
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					headers:   map[string]string{"test_header_": "test"},
					client: &http.Client{
						Timeout: 1 * time.Second,
					},
					zippers: newGzipPool(),
				},
				logger: zap.NewNop(),
			}

			numDroppedLogRecords, err := eventClient.pushResourceLogs(context.Background(), tt.resourceLogs)
			assert.Equal(t, tt.numDroppedLogRecords, numDroppedLogRecords)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestConsumeLogsDataWithAccessTokenPassthrough(t *testing.T) {
	fromHeaders := "AccessTokenFromClientHeaders"
	fromLabels := "AccessTokenFromLabel"

	newLogData := func(includeToken bool) pdata.ResourceLogs {
		out := makeSampleResourceLogs()

		if includeToken {
			res := out.Resource()
			res.Attributes().InsertString("com.splunk.signalfx.access_token", fromLabels)
		}
		return out
	}

	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		includedInLogData      bool
		expectedToken          string
	}{
		{
			name:                   "passthrough access token and included in logs",
			accessTokenPassthrough: true,
			includedInLogData:      true,
			expectedToken:          fromLabels,
		},
		{
			name:                   "passthrough access token and not included in logs",
			accessTokenPassthrough: true,
			includedInLogData:      false,
			expectedToken:          fromHeaders,
		},
		{
			name:                   "don't passthrough access token and included in logs",
			accessTokenPassthrough: false,
			includedInLogData:      true,
			expectedToken:          fromHeaders,
		},
		{
			name:                   "don't passthrough access token and not included in logs",
			accessTokenPassthrough: false,
			includedInLogData:      false,
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

			eventClient := &sfxEventClient{
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					headers: map[string]string{
						"test_header_": "test",
						"X-Sf-Token":   fromHeaders,
					},
					client: &http.Client{
						Timeout: 1 * time.Second,
					},
					zippers: newGzipPool(),
				},
				logger:                 zap.NewNop(),
				accessTokenPassthrough: tt.accessTokenPassthrough,
			}

			numDroppedLogRecords, err := eventClient.pushResourceLogs(context.Background(), newLogData(tt.includedInLogData))
			assert.Equal(t, 0, numDroppedLogRecords)
			assert.NoError(t, err)
		})
	}
}

func generateLargeDPBatch(t *testing.T) pdata.Metrics {
	md := pdata.NewMetrics()
	md.ResourceMetrics().Resize(6500)

	ts := time.Now()
	for i := 0; i < 6500; i++ {
		rm := md.ResourceMetrics().At(i)
		rm.InitEmpty()

		rm.InstrumentationLibraryMetrics().Resize(1)
		ilm := rm.InstrumentationLibraryMetrics().At(0)
		ilm.InitEmpty()
		ilm.Metrics()

		ilm.Metrics().Resize(1)
		m := ilm.Metrics().At(0)

		m.SetName("test_" + strconv.Itoa(i))
		m.SetDataType(pdata.MetricDataTypeIntGauge)
		m.IntGauge().InitEmpty()

		dp := pdata.NewIntDataPoint()
		dp.InitEmpty()
		dp.SetTimestamp(pdata.TimestampUnixNano(ts.UnixNano()))
		dp.LabelsMap().InitFromMap(map[string]string{
			"k0": "v0",
			"k1": "v1",
		})
		dp.SetValue(int64(i))

		m.IntGauge().DataPoints().Append(dp)
	}

	return md
}

func generateLargeEventBatch(t *testing.T) pdata.ResourceLogs {
	out := pdata.NewResourceLogs()
	out.InitEmpty()
	out.InstrumentationLibraryLogs().Resize(1)
	logs := out.InstrumentationLibraryLogs().At(0).Logs()

	batchSize := 65000
	logs.Resize(batchSize)
	ts := time.Now()
	for i := 0; i < batchSize; i++ {
		lr := logs.At(i)
		lr.SetName("test_" + strconv.Itoa(i))
		lr.Attributes().InsertString("k0", "k1")
		lr.Attributes().InsertNull("com.splunk.signalfx.event_category")
		lr.SetTimestamp(pdata.TimestampUnixNano(ts.UnixNano()))
	}

	return out
}

func TestConsumeMetadata(t *testing.T) {
	type args struct {
		metadata []*collection.MetadataUpdate
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
				[]*collection.MetadataUpdate{
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
				[]*collection.MetadataUpdate{
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
				[]*collection.MetadataUpdate{
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
				logger:       logger,
				pushMetadata: dimClient.PushMetadata,
			}
			sme := signalfMetadataExporter{
				pushMetadata: se.pushMetadata,
			}

			err = sme.ConsumeMetadata(tt.args.metadata)

			// Wait for requests to be handled by the mocked server before assertion.
			wg.Wait()

			require.NoError(t, err)
		})
	}
}

func BenchmarkExporterConsumeData(b *testing.B) {
	batchSize := 1000
	metrics := pdata.NewMetrics()
	for i := 0; i < batchSize; i++ {
		metrics.ResourceMetrics().Append(testMetricsData())
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	assert.NoError(b, err)

	dpClient := &sfxDPClient{
		sfxClientBase: sfxClientBase{
			ingestURL: serverURL,
			client: &http.Client{
				Timeout: 1 * time.Second,
			},
			zippers: sync.Pool{New: func() interface{} {
				return gzip.NewWriter(nil)
			}},
		},
		logger:    zap.NewNop(),
		converter: translation.NewMetricsConverter(zap.NewNop(), nil),
	}

	for i := 0; i < b.N; i++ {
		numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), metrics)
		assert.NoError(b, err)
		assert.Equal(b, 0, numDroppedTimeSeries)
	}
}

// Test to ensure SignalFx exporter implements collection.MetadataExporter in k8s_cluster receiver.
func TestSignalFxExporterConsumeMetadata(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	rCfg := cfg.(*Config)
	rCfg.AccessToken = "token"
	rCfg.Realm = "realm"
	exp, err := f.CreateMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, rCfg)
	require.NoError(t, err)

	kme, ok := exp.(collection.MetadataExporter)
	require.True(t, ok, "SignalFx exporter does not implement collection.MetadataExporter")
	require.NotNil(t, kme)
}
