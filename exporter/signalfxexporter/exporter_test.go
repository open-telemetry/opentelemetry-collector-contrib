// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"
	componentmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/utils"
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
				AccessToken:  "someToken",
				Realm:        "xyz",
				ClientConfig: confighttp.ClientConfig{Timeout: 1 * time.Second},
			},
		},
		{
			name: "create exporter with host metadata syncer",
			config: &Config{
				AccessToken:      "someToken",
				Realm:            "xyz",
				ClientConfig:     confighttp.ClientConfig{Timeout: 1 * time.Second},
				SyncHostMetadata: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSignalFxExporter(tt.config, exportertest.NewNopSettings(componentmetadata.Type))
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
	smallBatch := pmetric.NewMetrics()
	rm := smallBatch.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	m := ilm.Metrics().AppendEmpty()

	m.SetName("test_gauge")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.SetDoubleValue(123)

	tests := []struct {
		name                 string
		md                   pmetric.Metrics
		httpResponseCode     int
		retryAfter           int
		numDroppedTimeSeries int
		wantErr              bool
		wantPermanentErr     bool
		wantThrottleErr      bool
		wantStatusCode       int
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
			wantStatusCode:       403,
		},
		{
			name:                 "response_bad_request",
			md:                   smallBatch,
			httpResponseCode:     http.StatusBadRequest,
			numDroppedTimeSeries: 1,
			wantPermanentErr:     true,
			wantStatusCode:       400,
		},
		{
			name:                 "response_throttle",
			md:                   smallBatch,
			httpResponseCode:     http.StatusTooManyRequests,
			numDroppedTimeSeries: 1,
			wantThrottleErr:      true,
			wantStatusCode:       429,
		},
		{
			name:                 "response_throttle_with_header",
			md:                   smallBatch,
			retryAfter:           123,
			httpResponseCode:     http.StatusServiceUnavailable,
			numDroppedTimeSeries: 1,
			wantThrottleErr:      true,
			wantStatusCode:       503,
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
				_, _ = w.Write([]byte("response content"))
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			cfg := &Config{
				ClientConfig: confighttp.ClientConfig{
					Timeout: 1 * time.Second,
					Headers: map[string]configopaque.String{"test_header_": "test"},
				},
			}

			client, err := cfg.ToClient(context.Background(), componenttest.NewNopHost(), exportertest.NewNopSettings(componentmetadata.Type).TelemetrySettings)
			require.NoError(t, err)

			c, err := translation.NewMetricsConverter(zap.NewNop(), nil, nil, nil, "", false, true)
			require.NoError(t, err)
			require.NotNil(t, c)
			dpClient := &sfxDPClient{
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					client:    client,
					zippers: sync.Pool{New: func() any {
						return gzip.NewWriter(nil)
					}},
				},
				logger:    zap.NewNop(),
				converter: c,
			}

			errMsg := fmt.Sprintf("HTTP \"/v2/datapoint\" %d %q",
				tt.wantStatusCode,
				http.StatusText(tt.wantStatusCode),
			)

			numDroppedTimeSeries, err := dpClient.pushMetricsData(context.Background(), tt.md)
			assert.Equal(t, tt.numDroppedTimeSeries, numDroppedTimeSeries)

			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, errMsg)
				return
			}

			if tt.wantPermanentErr {
				errMsg = "Permanent error: " + errMsg
				assert.Error(t, err)
				assert.True(t, consumererror.IsPermanent(err))
				assert.EqualError(t, err, errMsg)
				return
			}

			if tt.wantThrottleErr {
				expected := errors.New(errMsg)
				expected = exporterhelper.NewThrottleRetry(expected, time.Duration(tt.retryAfter)*time.Second)
				assert.Equal(t, expected, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestConsumeMetricsWithAccessTokenPassthrough(t *testing.T) {
	fromHeaders := "AccessTokenFromClientHeaders"
	fromLabels := []string{"AccessTokenFromLabel0", "AccessTokenFromLabel1"}

	validMetricsWithToken := func(includeToken bool, token string, histogram bool) pmetric.Metrics {
		out := pmetric.NewMetrics()
		rm := out.ResourceMetrics().AppendEmpty()

		if includeToken {
			rm.Resource().Attributes().PutStr("com.splunk.signalfx.access_token", token)
		}

		ilm := rm.ScopeMetrics().AppendEmpty()
		m := ilm.Metrics().AppendEmpty()

		if histogram {
			buildHistogram(m, "test_histogram", pcommon.Timestamp(100000000), 1)
		} else {
			m.SetName("test_gauge")

			dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.Attributes().PutStr("k0", "v0")
			dp.Attributes().PutStr("k1", "v1")
			dp.SetDoubleValue(123)
		}

		return out
	}

	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		metrics                pmetric.Metrics
		additionalHeaders      map[string]string
		pushedTokens           []string
		sendOTLPHistograms     bool
	}{
		{
			name:                   "passthrough access token and included in md",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0], false),
			pushedTokens:           []string{fromLabels[0]},
			sendOTLPHistograms:     false,
		},
		{
			name:                   "passthrough access token and included in md with OTLP histogram",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0], true),
			pushedTokens:           []string{fromLabels[0]},
			sendOTLPHistograms:     true,
		},
		{
			name:                   "passthrough access token and not included in md",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(false, fromLabels[0], false),
			pushedTokens:           []string{fromHeaders},
			sendOTLPHistograms:     false,
		},
		{
			name:                   "passthrough access token and not included in md with OTLP histogram",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(false, fromLabels[0], true),
			pushedTokens:           []string{fromHeaders},
			sendOTLPHistograms:     true,
		},
		{
			name:                   "don't passthrough access token and included in md",
			accessTokenPassthrough: false,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], false)
				tgt := forFirstToken.ResourceMetrics().AppendEmpty()
				validMetricsWithToken(true, fromLabels[1], false).ResourceMetrics().At(0).CopyTo(tgt)
				return forFirstToken
			}(),
			pushedTokens:       []string{fromHeaders},
			sendOTLPHistograms: false,
		},
		{
			name:                   "don't passthrough access token and included in md with OTLP histogram",
			accessTokenPassthrough: false,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], true)
				tgt := forFirstToken.ResourceMetrics().AppendEmpty()
				validMetricsWithToken(true, fromLabels[1], true).ResourceMetrics().At(0).CopyTo(tgt)
				return forFirstToken
			}(),
			pushedTokens:       []string{fromHeaders},
			sendOTLPHistograms: true,
		},
		{
			name:                   "don't passthrough access token and not included in md",
			accessTokenPassthrough: false,
			metrics:                validMetricsWithToken(false, fromLabels[0], false),
			pushedTokens:           []string{fromHeaders},
			sendOTLPHistograms:     false,
		},
		{
			name:                   "don't passthrough access token and not included in md with OTLP histogram",
			accessTokenPassthrough: false,
			metrics:                validMetricsWithToken(false, fromLabels[0], true),
			pushedTokens:           []string{fromHeaders},
			sendOTLPHistograms:     true,
		},
		{
			name:                   "override user-specified token-like header",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0], false),
			additionalHeaders: map[string]string{
				"x-sf-token": "user-specified",
			},
			pushedTokens:       []string{fromLabels[0]},
			sendOTLPHistograms: false,
		},
		{
			name:                   "override user-specified token-like header with OTLP histogram",
			accessTokenPassthrough: true,
			metrics:                validMetricsWithToken(true, fromLabels[0], true),
			additionalHeaders: map[string]string{
				"x-sf-token": "user-specified",
			},
			pushedTokens:       []string{fromLabels[0]},
			sendOTLPHistograms: true,
		},
		{
			name:                   "use token from header when resource is nil",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				ilm := rm.ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()

				m.SetName("test_gauge")
				dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.Attributes().PutStr("k0", "v0")
				dp.Attributes().PutStr("k1", "v1")
				dp.SetDoubleValue(123)

				return out
			}(),
			pushedTokens:       []string{fromHeaders},
			sendOTLPHistograms: false,
		},
		{
			name:                   "use token from header when resource is nil with OTLP histogram",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				ilm := rm.ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				buildHistogram(m, "test_histogram", pcommon.Timestamp(1000), 1)
				return out
			}(),
			pushedTokens:       []string{fromHeaders},
			sendOTLPHistograms: true,
		},
		{
			name:                   "multiple tokens passed through",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], false)
				forSecondToken := validMetricsWithToken(true, fromLabels[1], false)
				forSecondToken.ResourceMetrics().EnsureCapacity(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())

				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromLabels[1]},
			sendOTLPHistograms: false,
		},
		{
			name:                   "multiple tokens passed through with OTLP histogram",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], true)
				forSecondToken := validMetricsWithToken(true, fromLabels[1], true)
				forSecondToken.ResourceMetrics().EnsureCapacity(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())

				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromLabels[1]},
			sendOTLPHistograms: true,
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[1], false)
				forSecondToken := validMetricsWithToken(true, fromLabels[0], false)
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1], false)

				forSecondToken.ResourceMetrics().EnsureCapacity(3)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())

				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromLabels[1]},
			sendOTLPHistograms: false,
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token with OTLP histogram",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[1], true)
				forSecondToken := validMetricsWithToken(true, fromLabels[0], true)
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1], true)

				forSecondToken.ResourceMetrics().EnsureCapacity(3)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())

				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromLabels[1]},
			sendOTLPHistograms: true,
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token grouped together",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], false)
				forSecondToken := validMetricsWithToken(true, fromLabels[1], false)
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1], false)

				forSecondToken.ResourceMetrics().EnsureCapacity(3)
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())

				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromLabels[1]},
			sendOTLPHistograms: false,
		},
		{
			name:                   "multiple tokens passed through - multiple md with same token grouped together in OTLP histogram",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], true)
				forSecondToken := validMetricsWithToken(true, fromLabels[1], true)
				moreForSecondToken := validMetricsWithToken(true, fromLabels[1], true)

				forSecondToken.ResourceMetrics().EnsureCapacity(3)
				moreForSecondToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())

				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromLabels[1]},
			sendOTLPHistograms: true,
		},
		{
			name:                   "multiple tokens passed through - one corrupted",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], false)
				forSecondToken := validMetricsWithToken(false, fromLabels[1], false)
				forSecondToken.ResourceMetrics().EnsureCapacity(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())
				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromHeaders},
			sendOTLPHistograms: false,
		},
		{
			name:                   "multiple tokens passed through - one corrupted in OTLP histogram",
			accessTokenPassthrough: true,
			metrics: func() pmetric.Metrics {
				forFirstToken := validMetricsWithToken(true, fromLabels[0], true)
				forSecondToken := validMetricsWithToken(false, fromLabels[1], true)
				forSecondToken.ResourceMetrics().EnsureCapacity(2)
				forFirstToken.ResourceMetrics().At(0).CopyTo(forSecondToken.ResourceMetrics().AppendEmpty())
				return forSecondToken
			}(),
			pushedTokens:       []string{fromLabels[0], fromHeaders},
			sendOTLPHistograms: true,
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
			cfg.Headers = make(map[string]configopaque.String)
			for k, v := range tt.additionalHeaders {
				cfg.Headers[k] = configopaque.String(v)
			}
			cfg.Headers["test_header_"] = configopaque.String(tt.name)
			cfg.AccessToken = configopaque.String(fromHeaders)
			cfg.AccessTokenPassthrough = tt.accessTokenPassthrough
			cfg.SendOTLPHistograms = tt.sendOTLPHistograms
			sfxExp, err := NewFactory().CreateMetrics(context.Background(), exportertest.NewNopSettings(componentmetadata.Type), cfg)
			require.NoError(t, err)
			require.NoError(t, sfxExp.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				require.NoError(t, sfxExp.Shutdown(context.Background()))
			}()

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

func TestConsumeMetricsAccessTokenPassthroughPriorityToContext(t *testing.T) {
	fromHeaders := "AccessTokenFromClientHeaders"
	fromLabels := []string{"AccessTokenFromLabel0", "AccessTokenFromLabel1"}
	fromContext := "AccessTokenFromContext"

	validMetricsWithToken := func(includeToken bool, token string, histogram bool) pmetric.Metrics {
		out := pmetric.NewMetrics()
		rm := out.ResourceMetrics().AppendEmpty()

		if includeToken {
			rm.Resource().Attributes().PutStr("com.splunk.signalfx.access_token", token)
		}

		ilm := rm.ScopeMetrics().AppendEmpty()
		m := ilm.Metrics().AppendEmpty()

		if histogram {
			buildHistogram(m, "test_histogram", pcommon.Timestamp(100000000), 1)
		} else {
			m.SetName("test_gauge")

			dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.Attributes().PutStr("k0", "v0")
			dp.Attributes().PutStr("k1", "v1")
			dp.SetDoubleValue(123)
		}

		return out
	}

	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		metrics                pmetric.Metrics
		additionalHeaders      map[string]string
		pushedTokens           []string
		sendOTLPHistograms     bool
		inContext              bool
	}{
		{
			name:                   "passthrough access token and included in md",
			accessTokenPassthrough: true,
			inContext:              true,
			metrics:                validMetricsWithToken(true, fromLabels[0], false),
			pushedTokens:           []string{fromContext},
		},
		{
			name:                   "passthrough access token and not included in md",
			accessTokenPassthrough: true,
			inContext:              true,
			metrics:                validMetricsWithToken(false, fromLabels[0], false),
			pushedTokens:           []string{fromContext},
			sendOTLPHistograms:     false,
		},
		{
			name:                   "passthrough access token and included in md",
			accessTokenPassthrough: true,
			inContext:              false,
			metrics:                validMetricsWithToken(true, fromLabels[0], false),
			pushedTokens:           []string{fromLabels[0]},
		},
		{
			name:                   "passthrough access token and not included in md",
			accessTokenPassthrough: true,
			inContext:              false,
			metrics:                validMetricsWithToken(false, fromLabels[0], false),
			pushedTokens:           []string{fromHeaders},
			sendOTLPHistograms:     false,
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
			cfg.Headers = make(map[string]configopaque.String)
			for k, v := range tt.additionalHeaders {
				cfg.Headers[k] = configopaque.String(v)
			}
			cfg.Headers["test_header_"] = configopaque.String(tt.name)
			cfg.AccessToken = configopaque.String(fromHeaders)
			cfg.AccessTokenPassthrough = tt.accessTokenPassthrough
			cfg.SendOTLPHistograms = tt.sendOTLPHistograms
			cfg.QueueSettings.Enabled = false
			sfxExp, err := NewFactory().CreateMetrics(context.Background(), exportertest.NewNopSettings(componentmetadata.Type), cfg)
			require.NoError(t, err)
			ctx := context.Background()
			if tt.inContext {
				ctx = client.NewContext(
					ctx,
					client.Info{Metadata: client.NewMetadata(
						map[string][]string{splunk.SFxAccessTokenHeader: {fromContext}},
					)},
				)
			}
			require.NoError(t, sfxExp.Start(ctx, componenttest.NewNopHost()))
			defer func() {
				require.NoError(t, sfxExp.Shutdown(context.Background()))
			}()

			err = sfxExp.ConsumeMetrics(ctx, tt.metrics)

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

func TestConsumeLogsAccessTokenPassthrough(t *testing.T) {
	fromHeaders := "AccessTokenFromClientHeaders"
	fromLabels := "AccessTokenFromLabel"
	fromContext := "AccessTokenFromContext"

	newLogData := func(includeToken bool) plog.Logs {
		out := makeSampleResourceLogs()
		makeSampleResourceLogs().ResourceLogs().At(0).CopyTo(out.ResourceLogs().AppendEmpty())

		if includeToken {
			out.ResourceLogs().At(0).Resource().Attributes().PutStr("com.splunk.signalfx.access_token", fromLabels)
			out.ResourceLogs().At(1).Resource().Attributes().PutStr("com.splunk.signalfx.access_token", fromLabels)
		}
		return out
	}

	tests := []struct {
		name                   string
		accessTokenPassthrough bool
		includedInLogData      bool
		inContext              bool
		expectedToken          string
	}{
		{
			name:                   "passthrough access token and not included in request context",
			inContext:              true,
			accessTokenPassthrough: true,
			includedInLogData:      true,
			expectedToken:          fromContext,
		},
		{
			name:                   "passthrough access token and included in logs",
			inContext:              false,
			accessTokenPassthrough: true,
			includedInLogData:      true,
			expectedToken:          fromLabels,
		},
		{
			name:                   "passthrough access token and not included in logs",
			inContext:              false,
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
			cfg.Headers = make(map[string]configopaque.String)
			cfg.Headers["test_header_"] = configopaque.String(tt.name)
			cfg.AccessToken = configopaque.String(fromHeaders)
			cfg.AccessTokenPassthrough = tt.accessTokenPassthrough
			cfg.QueueSettings.Enabled = false
			sfxExp, err := NewFactory().CreateLogs(context.Background(), exportertest.NewNopSettings(componentmetadata.Type), cfg)
			require.NoError(t, err)
			require.NoError(t, sfxExp.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				require.NoError(t, sfxExp.Shutdown(context.Background()))
			}()

			ctx := context.Background()
			if tt.inContext {
				ctx = client.NewContext(
					ctx,
					client.Info{Metadata: client.NewMetadata(
						map[string][]string{splunk.SFxAccessTokenHeader: {"AccessTokenFromContext"}},
					)},
				)
			}
			assert.NoError(t, sfxExp.ConsumeLogs(ctx, newLogData(tt.includedInLogData)))

			require.Eventually(t, func() bool {
				receivedTokens.Lock()
				defer receivedTokens.Unlock()
				return len(receivedTokens.tokens) == 1
			}, 1*time.Second, 10*time.Millisecond)
			assert.Equal(t, tt.expectedToken, receivedTokens.tokens[0])
		})
	}
}

func TestNewEventExporter(t *testing.T) {
	got, err := newEventExporter(nil, exportertest.NewNopSettings(componentmetadata.Type))
	assert.EqualError(t, err, "nil config")
	assert.Nil(t, got)

	got, err = newEventExporter(nil, exportertest.NewNopSettings(componentmetadata.Type))
	assert.Error(t, err)
	assert.Nil(t, got)

	cfg := &Config{
		AccessToken:  "someToken",
		Realm:        "xyz",
		ClientConfig: confighttp.ClientConfig{Timeout: 1 * time.Second},
	}

	got, err = newEventExporter(cfg, exportertest.NewNopSettings(componentmetadata.Type))
	assert.NoError(t, err)
	require.NotNil(t, got)

	err = got.startLogs(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	// This is expected to fail.
	ld := makeSampleResourceLogs()
	err = got.pushLogs(context.Background(), ld)
	assert.Error(t, err)
}

func makeSampleResourceLogs() plog.Logs {
	out := plog.NewLogs()
	l := out.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	l.SetTimestamp(pcommon.Timestamp(1000))
	attrs := l.Attributes()

	attrs.PutStr("k0", "v0")
	attrs.PutStr("k1", "v1")
	attrs.PutStr("k2", "v2")

	propMap := attrs.PutEmptyMap("com.splunk.signalfx.event_properties")
	propMap.PutStr("env", "prod")
	propMap.PutBool("isActive", true)
	propMap.PutInt("rack", 5)
	propMap.PutDouble("temp", 40.5)
	attrs.PutInt("com.splunk.signalfx.event_category", int64(sfxpb.EventCategory_USER_DEFINED))
	attrs.PutStr("com.splunk.signalfx.event_type", "shutdown")

	return out
}

func TestConsumeEventData(t *testing.T) {
	tests := []struct {
		name                 string
		resourceLogs         plog.Logs
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
			resourceLogs: func() plog.Logs {
				out := makeSampleResourceLogs()
				attrs := out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
				attrs.Remove("com.splunk.signalfx.event_category")
				attrs.Remove("com.splunk.signalfx.event_type")
				return out
			}(),
			reqTestFunc:          nil,
			numDroppedLogRecords: 1,
			httpResponseCode:     http.StatusAccepted,
		},
		{
			name: "nonconvertible_log_attrs",
			resourceLogs: func() plog.Logs {
				out := makeSampleResourceLogs()

				attrs := out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
				attrs.PutEmptyMap("map")

				propsAttrs, _ := attrs.Get("com.splunk.signalfx.event_properties")
				propsAttrs.Map().PutEmptyMap("map")

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

			cfg := &Config{
				ClientConfig: confighttp.ClientConfig{
					Timeout: 1 * time.Second,
					Headers: map[string]configopaque.String{"test_header_": "test"},
				},
			}

			client, err := cfg.ToClient(context.Background(), componenttest.NewNopHost(), exportertest.NewNopSettings(componentmetadata.Type).TelemetrySettings)
			require.NoError(t, err)

			eventClient := &sfxEventClient{
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					client:    client,
					zippers:   newGzipPool(),
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

	newLogData := func(includeToken bool) plog.Logs {
		out := makeSampleResourceLogs()
		makeSampleResourceLogs().ResourceLogs().At(0).CopyTo(out.ResourceLogs().AppendEmpty())

		if includeToken {
			out.ResourceLogs().At(0).Resource().Attributes().PutStr("com.splunk.signalfx.access_token", fromLabels)
			out.ResourceLogs().At(1).Resource().Attributes().PutStr("com.splunk.signalfx.access_token", fromLabels)
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
			cfg.Headers = make(map[string]configopaque.String)
			cfg.Headers["test_header_"] = configopaque.String(tt.name)
			cfg.AccessToken = configopaque.String(fromHeaders)
			cfg.AccessTokenPassthrough = tt.accessTokenPassthrough
			sfxExp, err := NewFactory().CreateLogs(context.Background(), exportertest.NewNopSettings(componentmetadata.Type), cfg)
			require.NoError(t, err)
			require.NoError(t, sfxExp.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				require.NoError(t, sfxExp.Shutdown(context.Background()))
			}()

			assert.NoError(t, sfxExp.ConsumeLogs(context.Background(), newLogData(tt.includedInLogData)))

			require.Eventually(t, func() bool {
				receivedTokens.Lock()
				defer receivedTokens.Unlock()
				return len(receivedTokens.tokens) == 1
			}, 1*time.Second, 10*time.Millisecond)
			assert.Equal(t, tt.expectedToken, receivedTokens.tokens[0])
		})
	}
}

func generateLargeDPBatch() pmetric.Metrics {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(6500)

	ts := time.Now()
	for i := 0; i < 6500; i++ {
		rm := md.ResourceMetrics().AppendEmpty()
		ilm := rm.ScopeMetrics().AppendEmpty()
		m := ilm.Metrics().AppendEmpty()

		m.SetName("test_" + strconv.Itoa(i))

		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.Attributes().PutStr("k0", "v0")
		dp.Attributes().PutStr("k1", "v1")
		dp.SetIntValue(int64(i))
	}

	return md
}

func generateLargeEventBatch() plog.Logs {
	out := plog.NewLogs()
	logs := out.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()

	batchSize := 65000
	logs.EnsureCapacity(batchSize)
	ts := time.Now()
	for i := 0; i < batchSize; i++ {
		lr := logs.AppendEmpty()
		lr.Attributes().PutStr("k0", "k1")
		lr.Attributes().PutEmpty("com.splunk.signalfx.event_category")
		lr.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}

	return out
}

func TestConsumeMetadataNotStarted(t *testing.T) {
	exporter := &signalfxExporter{}
	err := exporter.pushMetadata([]*metadata.MetadataUpdate{})
	require.ErrorContains(t, err, "exporter has not started")
}

func TestConsumeMetadata(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	converter, err := translation.NewMetricsConverter(
		zap.NewNop(),
		nil,
		cfg.ExcludeMetrics,
		cfg.IncludeMetrics,
		cfg.NonAlphanumericDimensionChars,
		false,
		true,
	)
	require.NoError(t, err)
	type args struct {
		metadata []*metadata.MetadataUpdate
	}
	type fields struct {
		payLoad map[string]any
	}
	tests := []struct {
		name                   string
		fields                 fields
		args                   args
		excludeProperties      []dpfilters.PropertyFilter
		expectedDimensionKey   string
		expectedDimensionValue string
		sendDelay              time.Duration
		shouldNotSendUpdate    bool
	}{
		{
			name: "Test property updates",
			fields: fields{
				map[string]any{
					"customProperties": map[string]any{
						"prop.erty1": "val1",
						"property2":  nil,
						"prop.erty3": "val33",
						"property4":  nil,
					},
					"tags":         nil,
					"tagsToRemove": nil,
				},
			},
			excludeProperties: []dpfilters.PropertyFilter{
				{
					DimensionName:  mustStringFilter(t, "/^.*$/"),
					DimensionValue: mustStringFilter(t, "/^.*$/"),
					PropertyName:   mustStringFilter(t, "/^property2$/"),
					PropertyValue:  mustStringFilter(t, "some*value"),
				},
				{
					DimensionName:  mustStringFilter(t, "/^.*$/"),
					DimensionValue: mustStringFilter(t, "/^.*$/"),
					PropertyName:   mustStringFilter(t, "property5"),
					PropertyValue:  mustStringFilter(t, "/^.*$/"),
				},
				{
					DimensionName:  mustStringFilter(t, "*"),
					DimensionValue: mustStringFilter(t, "*"),
					PropertyName:   mustStringFilter(t, "/^pro[op]erty6$/"),
					PropertyValue:  mustStringFilter(t, "property*value"),
				},
			},
			args: args{
				[]*metadata.MetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: metadata.MetadataDelta{
							MetadataToAdd: map[string]string{
								"prop.erty1": "val1",
								"property5":  "added.value",
								"property6":  "property6.value",
							},
							MetadataToRemove: map[string]string{
								"property2": "val2",
								"property5": "removed.value",
							},
							MetadataToUpdate: map[string]string{
								"prop.erty3": "val33",
								"property4":  "",
							},
						},
					},
				},
			},
			expectedDimensionKey:   "key",
			expectedDimensionValue: "id",
		},
		{
			name: "Test tag updates",
			fields: fields{
				map[string]any{
					"customProperties": map[string]any{},
					"tags": []any{
						"tag.1",
					},
					"tagsToRemove": []any{
						"tag/2",
					},
				},
			},
			excludeProperties: []dpfilters.PropertyFilter{
				{
					// confirms tags aren't affected by excludeProperties filters
					DimensionName:  mustStringFilter(t, "/^.*$/"),
					DimensionValue: mustStringFilter(t, "/^.*$/"),
					PropertyName:   mustStringFilter(t, "/^.*$/"),
					PropertyValue:  mustStringFilter(t, "/^.*$/"),
				},
			},
			args: args{
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
			expectedDimensionKey:   "key",
			expectedDimensionValue: "id",
		},
		{
			name: "Test quick successive updates",
			fields: fields{
				map[string]any{
					"customProperties": map[string]any{
						"property1": nil,
						"property2": "val2",
						"property3": nil,
					},
					"tags": []any{
						"tag/2",
					},
					"tagsToRemove": []any{
						"tag.1",
					},
				},
			},
			args: args{
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
			expectedDimensionKey:   "key",
			expectedDimensionValue: "id",
			sendDelay:              time.Second,
		},
		{
			name: "Test updates on dimensions with nonalphanumeric characters (other than the default allow list)",
			fields: fields{
				map[string]any{
					"customProperties": map[string]any{
						"prop.erty1": "val1",
						"property2":  nil,
						"prop.erty3": "val33",
						"property4":  nil,
					},
					"tags":         nil,
					"tagsToRemove": nil,
				},
			},
			args: args{
				[]*metadata.MetadataUpdate{
					{
						ResourceIDKey: "k!e=y",
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
			expectedDimensionKey:   "k_e_y",
			expectedDimensionValue: "id",
		},
		{
			name:                "no dimension update for empty properties",
			shouldNotSendUpdate: true,
			excludeProperties: []dpfilters.PropertyFilter{
				{
					DimensionName:  mustStringFilter(t, "key"),
					DimensionValue: mustStringFilter(t, "/^.*$/"),
					PropertyName:   mustStringFilter(t, "/^prop\\.erty[13]$/"),
					PropertyValue:  mustStringFilter(t, "/^.*$/"),
				},
				{
					DimensionName:  mustStringFilter(t, "*"),
					DimensionValue: mustStringFilter(t, "id"),
					PropertyName:   mustStringFilter(t, "property*"),
					PropertyValue:  mustStringFilter(t, "/^.*$/"),
				},
			},
			args: args{
				[]*metadata.MetadataUpdate{
					{
						ResourceIDKey: "key",
						ResourceID:    "id",
						MetadataDelta: metadata.MetadataDelta{
							MetadataToAdd: map[string]string{
								"prop.erty1": "val1",
								"property2":  "val2",
								"property5":  "added.value",
								"property6":  "property6.value",
							},
							MetadataToUpdate: map[string]string{
								"prop.erty3": "val33",
								"property4":  "val",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use WaitGroup to ensure the mocked server has encountered
			// a request from the exporter.
			wg := sync.WaitGroup{}
			wg.Add(1)

			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				// Test metadata updates are sent onto the right dimensions.
				dimPair := strings.Split(r.RequestURI, "/")[3:5]
				assert.Equal(t, tt.expectedDimensionKey, dimPair[0])
				assert.Equal(t, tt.expectedDimensionValue, dimPair[1])

				p := map[string]any{
					"customProperties": map[string]*string{},
					"tags":             []string{},
					"tagsToRemove":     []string{},
				}

				err = json.Unmarshal(b, &p)
				assert.NoError(t, err)

				assert.Equal(t, tt.fields.payLoad, p)
				wg.Done()
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			logger := zap.NewNop()

			dimClient := dimensions.NewDimensionClient(
				dimensions.DimensionClientOptions{
					Token:             "foo",
					APIURL:            serverURL,
					LogUpdates:        true,
					Logger:            logger,
					SendDelay:         tt.sendDelay,
					MaxBuffered:       10,
					MetricsConverter:  *converter,
					ExcludeProperties: tt.excludeProperties,
				})
			dimClient.Start()

			se := &signalfxExporter{
				dimClient: dimClient,
			}
			defer func() {
				_ = se.shutdown(context.Background())
			}()
			sme := signalfMetadataExporter{
				exporter: se,
			}

			err = sme.ConsumeMetadata(tt.args.metadata)
			c := make(chan struct{})
			go func() {
				defer close(c)
				wg.Wait()
			}()

			select {
			case <-c:
			// wait 500ms longer than send delay
			case <-time.After(tt.sendDelay + 500*time.Millisecond):
				// If no updates are supposed to be sent, the server doesn't update dimensions, and
				// doesn't call Done. This is correct behavior, so the test needs to account for it here,
				// or a goroutine will be leaked.
				defer wg.Done()
				require.True(t, tt.shouldNotSendUpdate, "timeout waiting for response")
			}

			require.NoError(t, err)
		})
	}
}

func BenchmarkExporterConsumeData(b *testing.B) {
	batchSize := 1000
	metrics := pmetric.NewMetrics()
	tmd := testMetricsData(false)
	for i := 0; i < batchSize; i++ {
		tmd.ResourceMetrics().At(0).CopyTo(metrics.ResourceMetrics().AppendEmpty())
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	assert.NoError(b, err)

	c, err := translation.NewMetricsConverter(zap.NewNop(), nil, nil, nil, "", false, true)
	require.NoError(b, err)
	require.NotNil(b, c)
	dpClient := &sfxDPClient{
		sfxClientBase: sfxClientBase{
			ingestURL: serverURL,
			client: &http.Client{
				Timeout: 1 * time.Second,
			},
			zippers: sync.Pool{New: func() any {
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
	exp, err := f.CreateMetrics(context.Background(), exportertest.NewNopSettings(componentmetadata.Type), rCfg)
	require.NoError(t, err)

	kme, ok := exp.(metadata.MetadataExporter)
	require.True(t, ok, "SignalFx exporter does not implement metadata.MetadataExporter")
	require.NotNil(t, kme)
}

func TestTLSExporterInit(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "valid CA",
			config: &Config{
				APIURL:    "https://test",
				IngestURL: "https://test",
				IngestTLSs: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "./testdata/certs/ca.pem",
					},
				},
				APITLSs: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "./testdata/certs/ca.pem",
					},
				},
				AccessToken:      "random",
				SyncHostMetadata: true,
			},
			wantErr: false,
		},
		{
			name: "missing CA",
			config: &Config{
				APIURL:    "https://test",
				IngestURL: "https://test",
				IngestTLSs: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "./testdata/certs/missingfile",
					},
				},
				AccessToken:      "random",
				SyncHostMetadata: true,
			},
			wantErr:        true,
			wantErrMessage: "failed to load CA CertPool",
		},
		{
			name: "invalid CA",
			config: &Config{
				APIURL:    "https://test",
				IngestURL: "https://test",
				IngestTLSs: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "./testdata/certs/invalid-ca.pem",
					},
				},
				AccessToken:      "random",
				SyncHostMetadata: true,
			},
			wantErr:        true,
			wantErrMessage: "failed to load CA CertPool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sfx, err := newSignalFxExporter(tt.config, exportertest.NewNopSettings(componentmetadata.Type))
			assert.NoError(t, err)
			err = sfx.start(context.Background(), componenttest.NewNopHost())
			defer func() { require.NoError(t, sfx.shutdown(context.Background())) }()
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMessage != "" {
					require.ErrorContains(t, err, tt.wantErrMessage)
				}
			} else {
				require.NotNil(t, sfx)
			}
		})
	}
}

func TestTLSIngestConnection(t *testing.T) {
	metricsPayload := pmetric.NewMetrics()
	rm := metricsPayload.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	m := ilm.Metrics().AppendEmpty()
	m.SetName("test_gauge")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.SetDoubleValue(123)

	server, err := newLocalHTTPSTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "connection is successful")
	}))
	require.NoError(t, err)
	defer server.Close()

	serverURL := server.URL

	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "Ingest CA not set",
			config: &Config{
				APIURL:           serverURL,
				IngestURL:        serverURL,
				AccessToken:      "random",
				SyncHostMetadata: true,
			},
			wantErr:        true,
			wantErrMessage: "x509.*certificate",
		},
		{
			name: "Ingest CA set",
			config: &Config{
				APIURL:    serverURL,
				IngestURL: serverURL,
				IngestTLSs: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "./testdata/certs/ca.pem",
					},
				},
				AccessToken:      "random",
				SyncHostMetadata: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sfx, err := newSignalFxExporter(tt.config, exportertest.NewNopSettings(componentmetadata.Type))
			assert.NoError(t, err)
			err = sfx.start(context.Background(), componenttest.NewNopHost())
			assert.NoError(t, err)
			defer func() { assert.NoError(t, sfx.shutdown(context.Background())) }()

			_, err = sfx.pushMetricsData(context.Background(), metricsPayload)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMessage != "" {
					assert.Regexp(t, tt.wantErrMessage, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultSystemCPUTimeExcludedAndTranslated(t *testing.T) {
	translator, err := translation.NewMetricTranslator(defaultTranslationRules, 3600, make(chan struct{}))
	require.NoError(t, err)
	converter, err := translation.NewMetricsConverter(zap.NewNop(), translator, defaultExcludeMetrics, nil, "_-.", false, true)
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("system.cpu.time")
	sum := m.SetEmptySum()
	for _, state := range []string{"idle", "interrupt", "nice", "softirq", "steal", "system", "user", "wait"} {
		for cpu := 0; cpu < 32; cpu++ {
			dp := sum.DataPoints().AppendEmpty()
			dp.SetDoubleValue(0)
			dp.Attributes().PutStr("cpu", fmt.Sprintf("%d", cpu))
			dp.Attributes().PutStr("state", state)
		}
	}
	dps := converter.MetricsToSignalFxV2(md)
	found := map[string]int64{}
	for _, dp := range dps {
		if dp.Metric == "cpu.num_processors" || dp.Metric == "cpu.idle" {
			intVal := dp.Value.IntValue
			require.NotNilf(t, intVal, "unexpected nil IntValue for %q", dp.Metric)
			found[dp.Metric] = *intVal
		} else {
			// account for unexpected w/ test-failing placeholder
			found[dp.Metric] = -1
		}
	}
	require.Equal(t, map[string]int64{
		"cpu.num_processors": 32,
		"cpu.idle":           0,
	}, found)
}

func TestTLSAPIConnection(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	converter, err := translation.NewMetricsConverter(
		zap.NewNop(),
		nil,
		cfg.ExcludeMetrics,
		cfg.IncludeMetrics,
		cfg.NonAlphanumericDimensionChars,
		false,
		true)
	require.NoError(t, err)

	metadata := []*metadata.MetadataUpdate{
		{
			ResourceIDKey: "key",
			ResourceID:    "id",
			MetadataDelta: metadata.MetadataDelta{
				MetadataToAdd: map[string]string{
					"prop.erty1": "val1",
				},
			},
		},
	}

	server, err := newLocalHTTPSTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "connection is successful")
	}))
	require.NoError(t, err)
	defer server.Close()

	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "API CA set",
			config: &Config{
				APIURL:           server.URL,
				IngestURL:        server.URL,
				AccessToken:      "random",
				SyncHostMetadata: true,
				APITLSs: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "./testdata/certs/ca.pem",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "API CA set",
			config: &Config{
				APIURL:           server.URL,
				IngestURL:        server.URL,
				AccessToken:      "random",
				SyncHostMetadata: true,
			},
			wantErr:        true,
			wantErrMessage: "error making HTTP request.*x509",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			logger := zap.New(observedZapCore)
			apiTLSCfg, err := tt.config.APITLSs.LoadTLSConfig(context.Background())
			require.NoError(t, err)
			serverURL, err := url.Parse(tt.config.APIURL)
			assert.NoError(t, err)
			dimClient := dimensions.NewDimensionClient(
				dimensions.DimensionClientOptions{
					Token:            "",
					APIURL:           serverURL,
					LogUpdates:       true,
					Logger:           logger,
					SendDelay:        1,
					MaxBuffered:      10,
					MetricsConverter: *converter,
					APITLSConfig:     apiTLSCfg,
				})
			dimClient.Start()
			defer func() { dimClient.Shutdown() }()

			se := &signalfxExporter{
				dimClient: dimClient,
			}
			sme := signalfMetadataExporter{
				exporter: se,
			}

			err = sme.ConsumeMetadata(metadata)
			time.Sleep(3 * time.Second)
			require.NoError(t, err)

			if tt.wantErr {
				if tt.wantErrMessage != "" {
					assert.Regexp(t, tt.wantErrMessage, observedLogs.All()[0].Context[0].Interface.(error).Error())
				}
			} else {
				require.Equal(t, 1, observedLogs.Len())
				require.Nil(t, observedLogs.All()[0].Context[0].Interface)
			}
		})
	}
}

func newLocalHTTPSTestServer(handler http.Handler) (*httptest.Server, error) {
	ts := httptest.NewUnstartedServer(handler)
	cert, err := tls.LoadX509KeyPair("./testdata/certs/cert.pem", "./testdata/certs/cert-key.pem")
	if err != nil {
		return nil, err
	}
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	ts.StartTLS()
	return ts, nil
}

func BenchmarkExporterConsumeDataWithOTLPHistograms(b *testing.B) {
	batchSize := 1000
	metrics := pmetric.NewMetrics()
	tmd := testMetricsData(true)
	for i := 0; i < batchSize; i++ {
		tmd.ResourceMetrics().At(0).CopyTo(metrics.ResourceMetrics().AppendEmpty())
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	assert.NoError(b, err)

	c, err := translation.NewMetricsConverter(zap.NewNop(), nil, nil, nil, "", false, false)
	require.NoError(b, err)
	require.NotNil(b, c)
	dpClient := &sfxDPClient{
		sfxClientBase: sfxClientBase{
			ingestURL: serverURL,
			client: &http.Client{
				Timeout: 1 * time.Second,
			},
			zippers: sync.Pool{New: func() any {
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

func TestConsumeMixedMetrics(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Now())
	smallBatch := pmetric.NewMetrics()
	rm := smallBatch.ResourceMetrics().AppendEmpty()
	res := rm.Resource()
	res.Attributes().PutStr("kr0", "vr0")
	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()
	ilm.Scope().Attributes().PutStr("ks0", "vs0")
	ilm.Metrics().EnsureCapacity(2)
	ilm.Metrics().AppendEmpty()
	buildHistogram(ilm.Metrics().At(0), "test_histogram", ts, 2)
	ilm.Metrics().AppendEmpty()
	m1 := ilm.Metrics().At(1)
	m1.SetName("test_gauge")
	dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
	dp1.Attributes().PutStr("k0", "v0")
	dp1.SetDoubleValue(123)

	smallBatchHistogramOnly := pmetric.NewMetrics()
	rmh := smallBatchHistogramOnly.ResourceMetrics().AppendEmpty()
	resh := rmh.Resource()
	resh.Attributes().PutStr("kr0", "vr0")
	ilmsh := rmh.ScopeMetrics()
	ilmsh.EnsureCapacity(2)
	ilmh := ilmsh.AppendEmpty()
	ilmh.Scope().Attributes().PutStr("ks0", "vs0")
	ilmh.Metrics().EnsureCapacity(2)
	ilmh.Metrics().AppendEmpty()
	buildHistogram(ilmh.Metrics().At(0), "test_histogram", ts, 2)

	tests := []struct {
		name                  string
		md                    pmetric.Metrics
		sfxHTTPResponseCode   int
		otlpHTTPResponseCode  int
		retryAfter            int
		numDroppedTimeSeries  int
		wantErr               bool
		wantPermanentErr      bool
		wantThrottleErr       bool
		wantPartialMetricsErr bool
		wantStatusCode        int
	}{
		{
			name:                 "happy_path",
			md:                   smallBatch,
			sfxHTTPResponseCode:  http.StatusAccepted,
			otlpHTTPResponseCode: http.StatusAccepted,
		},
		{
			name:                 "happy_path_otlp",
			md:                   smallBatchHistogramOnly,
			otlpHTTPResponseCode: http.StatusAccepted,
		},
		{
			name:                 "response_forbidden_sfx",
			md:                   smallBatch,
			sfxHTTPResponseCode:  http.StatusForbidden,
			numDroppedTimeSeries: 1,
			wantErr:              true,
			wantStatusCode:       403,
		},
		{
			name:                 "response_forbidden_otlp",
			md:                   smallBatchHistogramOnly,
			otlpHTTPResponseCode: http.StatusForbidden,
			numDroppedTimeSeries: 2,
			wantErr:              true,
			wantStatusCode:       403,
		},
		{
			name:                 "response_forbidden_mixed",
			md:                   smallBatch,
			sfxHTTPResponseCode:  http.StatusAccepted,
			otlpHTTPResponseCode: http.StatusForbidden,
			numDroppedTimeSeries: 2,
			wantErr:              true,
			wantStatusCode:       403,
		},
		{
			name:                 "response_bad_request_sfx",
			md:                   smallBatch,
			sfxHTTPResponseCode:  http.StatusBadRequest,
			numDroppedTimeSeries: 1,
			wantPermanentErr:     true,
			wantStatusCode:       400,
		},
		{
			name:                 "response_bad_request_otlp",
			md:                   smallBatchHistogramOnly,
			otlpHTTPResponseCode: http.StatusBadRequest,
			numDroppedTimeSeries: 2,
			wantPermanentErr:     true,
			wantStatusCode:       400,
		},
		{
			name:                 "response_bad_request_mixed",
			md:                   smallBatch,
			sfxHTTPResponseCode:  http.StatusAccepted,
			otlpHTTPResponseCode: http.StatusBadRequest,
			numDroppedTimeSeries: 2,
			wantPermanentErr:     true,
			wantStatusCode:       400,
		},
		{
			name:                 "response_throttle_sfx",
			md:                   smallBatch,
			sfxHTTPResponseCode:  http.StatusTooManyRequests,
			numDroppedTimeSeries: 1,
			wantThrottleErr:      true,
			wantStatusCode:       429,
		},
		{
			name:                  "response_throttle_mixed",
			md:                    smallBatch,
			sfxHTTPResponseCode:   http.StatusAccepted,
			otlpHTTPResponseCode:  http.StatusTooManyRequests,
			numDroppedTimeSeries:  2,
			wantThrottleErr:       true,
			wantPartialMetricsErr: true,
			wantStatusCode:        429,
		},
		{
			name:                  "response_throttle_otlp",
			md:                    smallBatchHistogramOnly,
			otlpHTTPResponseCode:  http.StatusTooManyRequests,
			numDroppedTimeSeries:  2,
			wantThrottleErr:       true,
			wantPartialMetricsErr: true,
			wantStatusCode:        429,
		},
		{
			name:                 "response_throttle_with_header_sfx",
			md:                   smallBatch,
			retryAfter:           123,
			sfxHTTPResponseCode:  http.StatusServiceUnavailable,
			numDroppedTimeSeries: 1,
			wantThrottleErr:      true,
			wantStatusCode:       503,
		},
		{
			name:                  "response_throttle_with_header_otlp",
			md:                    smallBatchHistogramOnly,
			retryAfter:            123,
			otlpHTTPResponseCode:  http.StatusServiceUnavailable,
			numDroppedTimeSeries:  2,
			wantThrottleErr:       true,
			wantPartialMetricsErr: true,
			wantStatusCode:        503,
		},
		{
			name:                  "response_throttle_with_header_mixed",
			md:                    smallBatch,
			retryAfter:            123,
			sfxHTTPResponseCode:   http.StatusAccepted,
			otlpHTTPResponseCode:  http.StatusServiceUnavailable,
			numDroppedTimeSeries:  2,
			wantThrottleErr:       true,
			wantPartialMetricsErr: true,
			wantStatusCode:        503,
		},
		{
			name:                 "large_batch",
			md:                   generateLargeMixedDPBatch(),
			sfxHTTPResponseCode:  http.StatusAccepted,
			otlpHTTPResponseCode: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "test", r.Header.Get("test_header_"))
				var respCode int
				if r.Header.Get("Content-Type") == otlpProtobufContentType {
					respCode = tt.otlpHTTPResponseCode
				} else {
					respCode = tt.sfxHTTPResponseCode
				}
				if (respCode == http.StatusTooManyRequests ||
					respCode == http.StatusServiceUnavailable) && tt.retryAfter != 0 {
					w.Header().Add(splunk.HeaderRetryAfter, strconv.Itoa(tt.retryAfter))
				}
				w.WriteHeader(respCode)
				_, _ = w.Write([]byte("response content"))
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			assert.NoError(t, err)

			cfg := &Config{
				ClientConfig: confighttp.ClientConfig{
					Timeout: 1 * time.Second,
					Headers: map[string]configopaque.String{"test_header_": "test"},
				},
			}

			client, err := cfg.ToClient(context.Background(), componenttest.NewNopHost(), exportertest.NewNopSettings(componentmetadata.Type).TelemetrySettings)
			require.NoError(t, err)

			c, err := translation.NewMetricsConverter(zap.NewNop(), nil, nil, nil, "", false, false)
			require.NoError(t, err)
			require.NotNil(t, c)
			sfxClient := &sfxDPClient{
				sfxClientBase: sfxClientBase{
					ingestURL: serverURL,
					client:    client,
					zippers: sync.Pool{New: func() any {
						return gzip.NewWriter(nil)
					}},
				},
				logger:             zap.NewNop(),
				converter:          c,
				sendOTLPHistograms: true,
			}

			numDroppedTimeSeries, err := sfxClient.pushMetricsData(context.Background(), tt.md)
			assert.Equal(t, tt.numDroppedTimeSeries, numDroppedTimeSeries)

			errMsg := fmt.Sprintf("HTTP \"/v2/datapoint\" %d %q",
				tt.wantStatusCode,
				http.StatusText(tt.wantStatusCode),
			)

			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, errMsg)
				return
			}

			if tt.wantPermanentErr {
				errMsg = "Permanent error: " + errMsg
				assert.Error(t, err)
				assert.True(t, consumererror.IsPermanent(err))
				assert.EqualError(t, err, errMsg)
				return
			}

			if tt.wantThrottleErr {
				if tt.wantPartialMetricsErr {
					partialMetrics, _ := utils.GetHistograms(smallBatch)
					throttleErr := errors.New(errMsg)
					throttleErr = exporterhelper.NewThrottleRetry(throttleErr, time.Duration(tt.retryAfter)*time.Second)
					testErr := consumererror.NewMetrics(throttleErr, partialMetrics)
					assert.Equal(t, testErr, err)
					return
				}

				expected := errors.New(errMsg)
				expected = exporterhelper.NewThrottleRetry(expected, time.Duration(tt.retryAfter)*time.Second)
				assert.Equal(t, expected, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func generateLargeMixedDPBatch() pmetric.Metrics {
	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(7500)

	ts := pcommon.NewTimestampFromTime(time.Now())
	for i := 0; i < 7500; i++ {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("kr0", "vr0")
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Metrics().EnsureCapacity(2)
		m1 := ilm.Metrics().AppendEmpty()
		m1.SetName("test_" + strconv.Itoa(i))
		dp := m1.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(ts)
		dp.Attributes().PutStr("k0", "v0")
		dp.Attributes().PutStr("k1", "v1")
		dp.SetIntValue(int64(i))
		m2 := ilm.Metrics().AppendEmpty()
		buildHistogram(m2, "histogram_"+strconv.Itoa(i), ts, 1)
	}
	return md
}
