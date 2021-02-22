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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/dimensions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
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
			name: "fails to create metrics converter",
			config: &Config{
				AccessToken:    "test",
				Realm:          "realm",
				ExcludeMetrics: []dpfilters.MetricFilter{{}},
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
	rm.InstrumentationLibraryMetrics().Resize(1)
	ilm := rm.InstrumentationLibraryMetrics().At(0)
	ilm.Metrics().Resize(1)
	m := ilm.Metrics().At(0)

	m.SetName("test_gauge")
	m.SetDataType(pdata.MetricDataTypeDoubleGauge)
	m.DoubleGauge().DataPoints().Resize(1)
	dp := m.DoubleGauge().DataPoints().At(0)
	dp.LabelsMap().InitFromMap(map[string]string{
		"k0": "v0",
		"k1": "v1",
	})
	dp.SetValue(123)

	tests := []struct {
		name                 string
		md                   pdata.Metrics
		httpResponseCode     int
		retryAfter           int
		numDroppedTimeSeries int
		wantErr              bool
		wantPermanentErr     bool
		wantThrottleErr      bool
	}{
		{
			name:             "happy_path",
			md:               smallBatch,
			httpResponseCode: http.StatusAccepted,
		},
		{
			name:                 "response_forbidden",
			md:                   smallBatch,
			httpResponseCode:     http.StatusForbidden,
			numDroppedTimeSeries: 1,
			wantErr:              true,
		},
		{
			name:                 "response_bad_request",
			md:                   smallBatch,
			httpResponseCode:     http.StatusBadRequest,
			numDroppedTimeSeries: 1,
			wantPermanentErr:     true,
		},
		{
			name:                 "response_throttle",
			md:                   smallBatch,
			httpResponseCode:     http.StatusTooManyRequests,
			numDroppedTimeSeries: 1,
			wantThrottleErr:      true,
		},
		{
			name:                 "response_throttle_with_header",
			md:                   smallBatch,
			retryAfter:           123,
			httpResponseCode:     http.StatusServiceUnavailable,
			numDroppedTimeSeries: 1,
			wantThrottleErr:      true,
		},
		{
			name:             "large_batch",
			md:               generateLargeDPBatch(),
			httpResponseCode: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "test", r.Header.Get("test_header_"))
				if (tt.httpResponseCode == http.StatusTooManyRequests ||
					tt.httpResponseCode == http.StatusServiceUnavailable) && tt.retryAfter != 0 {
					w.Header().Add(splunk.HeaderRetryAfter, strconv.Itoa(tt.retryAfter))
				}
				w.WriteHeader(tt.httpResponseCode)
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			c, err := translation.NewMetricsConverter(zap.NewNop(), nil, nil, nil, "")
			require.NoError(t, err)
			require.NotNil(t, c)
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
				converter: c,
			}

			numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), tt.md)
			assert.Equal(t, tt.numDroppedTimeSeries, numDroppedTimeSeries)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			if tt.wantPermanentErr {
				assert.Error(t, err)
				assert.True(t, consumererror.IsPermanent(err))
				return
			}

			if tt.wantThrottleErr {
				expected := fmt.Errorf("HTTP %d %q", tt.httpResponseCode, http.StatusText(tt.httpResponseCode))
				expected = exporterhelper.NewThrottleRetry(expected, time.Duration(tt.retryAfter)*time.Second)
				assert.EqualValues(t, expected, err)
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

		if includeToken {
			rm.Resource().Attributes().InitFromMap(map[string]pdata.AttributeValue{
				"com.splunk.signalfx.access_token": pdata.NewAttributeValueString(token),
			})
		}

		rm.InstrumentationLibraryMetrics().Resize(1)
		ilm := rm.InstrumentationLibraryMetrics().At(0)
		ilm.Metrics().Resize(1)
		m := ilm.Metrics().At(0)

		m.SetName("test_gauge")
		m.SetDataType(pdata.MetricDataTypeDoubleGauge)

		m.DoubleGauge().DataPoints().Resize(1)
		dp := m.DoubleGauge().DataPoints().At(0)
		dp.LabelsMap().InitFromMap(map[string]string{
			"k0": "v0",
			"k1": "v1",
		})
		dp.SetValue(123)
		return out
	}

	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		metrics                pdata.Metrics
		additionalHeaders      map[string]string
		pushedTokens           []string
	}{
		{
			name:                   "passthrough access token and included in md",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0]),
			pushedTokens:           []string{fromLabels[0]},
		},
		{
			name:                   "passthrough access token and not included in md",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(false, fromLabels[0]),
			pushedTokens:           []string{fromHeaders},
		},
		{
			name:                   "don't passthrough access token and included in md",
			accessTokenPassthrough: false,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0])
				forFirstToken.ResourceMetrics().Append(validMetricsWithToken(true, fromLabels[1]).ResourceMetrics().At(0))
				return forFirstToken
			}(),
			pushedTokens: []string{fromHeaders},
		},
		{
			name:                   "don't passthrough access token and not included in md",
			accessTokenPassthrough: false,
			metrics:                validMetricsWithToken(false, fromLabels[0]),
			pushedTokens:           []string{fromHeaders},
		},
		{
			name:                   "override user-specified token-like header",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0]),
			additionalHeaders: map[string]string{
				"x-sf-token": "user-specified",
			},
			pushedTokens: []string{fromLabels[0]},
		},
		{
			name:                   "use token from header when resource is nil",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				out := pdata.NewMetrics()
				out.ResourceMetrics().Resize(1)
				rm := out.ResourceMetrics().At(0)

				rm.InstrumentationLibraryMetrics().Resize(1)
				ilm := rm.InstrumentationLibraryMetrics().At(0)
				ilm.Metrics().Resize(1)
				m := ilm.Metrics().At(0)

				m.SetName("test_gauge")
				m.SetDataType(pdata.MetricDataTypeDoubleGauge)
				m.DoubleGauge().DataPoints().Resize(1)
				dp := m.DoubleGauge().DataPoints().At(0)
				dp.LabelsMap().InitFromMap(map[string]string{
					"k0": "v0",
					"k1": "v1",
				})
				dp.SetValue(123)

				return out
			}(),
			pushedTokens: []string{fromHeaders},
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
			pushedTokens: []string{fromLabels[0], fromLabels[1]},
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token",
			accessTokenPassthrough: true,
			metrics: func() pdata.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[1])
				forSecondToken := validMetricsWithToken(true, fromLabels[0])
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1])

				forSecondToken.ResourceMetrics().Resize(3)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(1))
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().At(2))

				return forSecondToken
			}(),
			pushedTokens: []string{fromLabels[0], fromLabels[1]},
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
			pushedTokens: []string{fromLabels[0], fromLabels[1]},
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
			pushedTokens: []string{fromLabels[0], fromHeaders},
		},
	}
	for _, tt := range tests {
		receivedTokens := struct {
			sync.Mutex
			tokens []string
		}{}
		receivedTokens.tokens = []string{}
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.name, r.Header.Get("test_header_"))
				receivedTokens.Lock()

				token := r.Header.Get("x-sf-token")
				receivedTokens.tokens = append(receivedTokens.tokens, token)

				receivedTokens.Unlock()
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.IngestURL = server.URL
			cfg.APIURL = server.URL
			cfg.Headers = make(map[string]string)
			for k, v := range tt.additionalHeaders {
				cfg.Headers[k] = v
			}
			cfg.Headers["test_header_"] = tt.name
			cfg.AccessToken = fromHeaders
			cfg.AccessTokenPassthrough = tt.accessTokenPassthrough
			sfxExp, err := NewFactory().CreateMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, cfg)
			require.NoError(t, err)
			require.NoError(t, sfxExp.Start(context.Background(), componenttest.NewNopHost()))
			defer sfxExp.Shutdown(context.Background())

			err = sfxExp.ConsumeMetrics(context.Background(), tt.metrics)

			assert.NoError(t, err)
			require.Eventually(t, func() bool {
				receivedTokens.Lock()
				defer receivedTokens.Unlock()
				return len(tt.pushedTokens) == len(receivedTokens.tokens)
			}, 1*time.Second, 10*time.Millisecond)
			sort.Strings(tt.pushedTokens)
			sort.Strings(receivedTokens.tokens)
			assert.Equal(t, tt.pushedTokens, receivedTokens.tokens)
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
	ld := makeSampleResourceLogs()
	_, err = got.pushLogs(context.Background(), ld)
	assert.Error(t, err)
}

func makeSampleResourceLogs() pdata.Logs {
	out := pdata.NewLogs()
	out.ResourceLogs().Resize(1)
	out.ResourceLogs().At(0).InstrumentationLibraryLogs().Resize(1)

	logSlice := out.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs()
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

	return out
}

func TestConsumeEventData(t *testing.T) {
	tests := []struct {
		name                 string
		resourceLogs         pdata.Logs
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
			resourceLogs: func() pdata.Logs {
				out := makeSampleResourceLogs()
				out.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().Delete("com.splunk.signalfx.event_category")
				return out
			}(),
			reqTestFunc:          nil,
			numDroppedLogRecords: 1,
			httpResponseCode:     http.StatusAccepted,
		},
		{
			name: "nonconvertible_log_attrs",
			resourceLogs: func() pdata.Logs {
				out := makeSampleResourceLogs()

				attrs := out.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes()
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
			resourceLogs:     generateLargeEventBatch(),
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

			numDroppedLogRecords, err := eventClient.pushLogsData(context.Background(), tt.resourceLogs)
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

	newLogData := func(includeToken bool) pdata.Logs {
		out := makeSampleResourceLogs()
		out.ResourceLogs().Append(makeSampleResourceLogs().ResourceLogs().At(0))

		if includeToken {
			out.ResourceLogs().At(0).Resource().Attributes().InsertString("com.splunk.signalfx.access_token", fromLabels)
			out.ResourceLogs().At(1).Resource().Attributes().InsertString("com.splunk.signalfx.access_token", fromLabels)
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
			receivedTokens := struct {
				sync.Mutex
				tokens []string
			}{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.name, r.Header.Get("test_header_"))
				receivedTokens.Lock()
				receivedTokens.tokens = append(receivedTokens.tokens, r.Header.Get("x-sf-token"))
				receivedTokens.Unlock()
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.IngestURL = server.URL
			cfg.APIURL = server.URL
			cfg.Headers = make(map[string]string)
			cfg.Headers["test_header_"] = tt.name
			cfg.AccessToken = fromHeaders
			cfg.AccessTokenPassthrough = tt.accessTokenPassthrough
			sfxExp, err := NewFactory().CreateLogsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, cfg)
			require.NoError(t, err)
			require.NoError(t, sfxExp.Start(context.Background(), componenttest.NewNopHost()))
			defer sfxExp.Shutdown(context.Background())

			assert.NoError(t, sfxExp.ConsumeLogs(context.Background(), newLogData(tt.includedInLogData)))

			require.Eventually(t, func() bool {
				receivedTokens.Lock()
				defer receivedTokens.Unlock()
				return len(receivedTokens.tokens) == 1
			}, 1*time.Second, 10*time.Millisecond)
			assert.Equal(t, receivedTokens.tokens[0], tt.expectedToken)
		})
	}
}

func generateLargeDPBatch() pdata.Metrics {
	md := pdata.NewMetrics()
	md.ResourceMetrics().Resize(6500)

	ts := time.Now()
	for i := 0; i < 6500; i++ {
		rm := md.ResourceMetrics().At(i)

		rm.InstrumentationLibraryMetrics().Resize(1)
		ilm := rm.InstrumentationLibraryMetrics().At(0)
		ilm.Metrics().Resize(1)
		m := ilm.Metrics().At(0)

		m.SetName("test_" + strconv.Itoa(i))
		m.SetDataType(pdata.MetricDataTypeIntGauge)

		dp := pdata.NewIntDataPoint()
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

func generateLargeEventBatch() pdata.Logs {
	out := pdata.NewLogs()
	out.ResourceLogs().Resize(1)
	out.ResourceLogs().At(0).InstrumentationLibraryLogs().Resize(1)
	logs := out.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs()

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
		metadata []*metadata.MetadataUpdate
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
				[]*metadata.MetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: metadata.MetadataDelta{
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
				[]*metadata.MetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: metadata.MetadataDelta{
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
				[]*metadata.MetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: metadata.MetadataDelta{
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
						MetadataDelta: metadata.MetadataDelta{
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
						MetadataDelta: metadata.MetadataDelta{
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

	c, err := translation.NewMetricsConverter(zap.NewNop(), nil, nil, nil, "")
	require.NoError(b, err)
	require.NotNil(b, c)
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
		converter: c,
	}

	for i := 0; i < b.N; i++ {
		numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), metrics)
		assert.NoError(b, err)
		assert.Equal(b, 0, numDroppedTimeSeries)
	}
}

// Test to ensure SignalFx exporter implements metadata.MetadataExporter in k8s_cluster receiver.
func TestSignalFxExporterConsumeMetadata(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	rCfg := cfg.(*Config)
	rCfg.AccessToken = "token"
	rCfg.Realm = "realm"
	exp, err := f.CreateMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, rCfg)
	require.NoError(t, err)

	kme, ok := exp.(metadata.MetadataExporter)
	require.True(t, ok, "SignalFx exporter does not implement metadata.MetadataExporter")
	require.NotNil(t, kme)
}
