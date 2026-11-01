// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

type fakeTransport struct {
	resp *http.Response
	err  error
}

func (f fakeTransport) Perform(*http.Request) (*http.Response, error) {
	return f.resp, f.err
}

func TestFetchESInfo(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		tp                   *fakeTransport
		expectedVersion      string
		expectedBuildFlavour string
		expectedError        string
	}{
		{
			name: "returns the correct version and build flavor",
			tp: &fakeTransport{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"version":{"number":"9.6.0","build_flavor":"default"}}`)),
					Header:     make(http.Header),
				},
			},
			expectedVersion:      "9.6.0",
			expectedBuildFlavour: "default",
		},
		{
			name: "returns error when http status code is not 200",
			tp: &fakeTransport{
				resp: &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       http.NoBody,
					Header:     make(http.Header),
				},
			},
			expectedError: "es info returned status 500",
		},
		{
			name: "returns error when decoder fails",
			tp: &fakeTransport{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"version":{"number":"9.6.0",}}`)),
					Header:     make(http.Header),
				},
			},
			expectedError: "decoding es info: invalid character '}' looking for beginning of object key string",
		},
		{
			name:          "returns error when transport fails",
			tp:            &fakeTransport{err: errors.New("boom")},
			expectedError: "fetching es info: boom",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var info esInfo
			err := info.fetchESInfo(t.Context(), tc.tp)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedVersion, info.Version.Number)
			assert.Equal(t, tc.expectedBuildFlavour, info.Version.BuildFlavor)
		})
	}
}

func TestGetElasticsearchVersions(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		handler               http.HandlerFunc
		authConfig            *AuthenticationSettings
		expectConnected       bool
		expectSemverParseFail bool
		expectedVersion       string
		expectedFlavor        string
	}{
		{
			name: "returns and logs version and build flavor on success",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version":{"number":"9.6.0","build_flavor":"default"}}`))
			},
			expectConnected: true,
			expectedVersion: "9.6.0",
			expectedFlavor:  "default",
		},
		{
			name: "returns and logs version and build flavor for snapshot on success",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version":{"number":"9.6.0-SNAPSHOT","build_flavor":"default"}}`))
			},
			expectConnected: true,
			expectedVersion: "9.6.0-SNAPSHOT",
			expectedFlavor:  "default",
		},
		{
			name: "warns and does not log connection when info fetch fails",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"boom"}`))
			},
			expectConnected: false,
		},
		{
			name: "warns that version number could not be parsed",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version":{"number":"bad-version","build_flavor":"default"}}`))
			},
			expectConnected:       true,
			expectSemverParseFail: true,
		},
		{
			name: "connects using configured API key",
			handler: func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth != "APIKey test-123" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version":{"number":"9.6.0-SNAPSHOT","build_flavor":"default"}}`))
			},
			authConfig: &AuthenticationSettings{
				APIKey: configopaque.String("test-123"),
			},
			expectConnected: true,
			expectedVersion: "9.6.0-SNAPSHOT",
			expectedFlavor:  "default",
		},
		{
			name: "connects using configured username and password",
			handler: func(w http.ResponseWriter, r *http.Request) {
				uname, pass, ok := r.BasicAuth()
				if !ok || uname != "elastic" || pass != "changeme" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version":{"number":"9.6.0-SNAPSHOT","build_flavor":"default"}}`))
			},
			authConfig: &AuthenticationSettings{
				User:     "elastic",
				Password: configopaque.String("changeme"),
			},
			expectConnected: true,
			expectedVersion: "9.6.0-SNAPSHOT",
			expectedFlavor:  "default",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)

			core, observed := observer.New(zapcore.InfoLevel)
			set := exportertest.NewNopSettings(metadata.Type)
			set.Logger = zap.New(core)

			cfg := createDefaultConfig().(*Config)
			cfg.Endpoints = []string{srv.URL}
			if tc.authConfig != nil {
				cfg.Authentication = *tc.authConfig
			}

			actualVersion := getElasticsearchVersions(t.Context(), cfg, set, componenttest.NewNopHost())

			connected := observed.FilterMessage("Connected to Elasticsearch").All()
			if !tc.expectConnected {
				assert.Empty(t, connected)
				assert.NotEmpty(t, observed.FilterMessage("failed to fetch Elasticsearch info").All())
				return
			}

			if tc.expectSemverParseFail {
				assert.NotEmpty(t, observed.FilterMessage("could not parse Elasticsearch version").All())
				assert.Nil(t, actualVersion)
				assert.Empty(t, connected)
				return
			}

			require.Len(t, connected, 1)
			fields := connected[0].ContextMap()
			assert.Equal(t, tc.expectedVersion, fields["version"])
			assert.Equal(t, tc.expectedFlavor, fields["build_flavor"])
			assert.Equal(t, srv.URL, fields["endpoint"])

			assert.Equal(t, tc.expectedVersion, actualVersion.String())
		})
	}
}

func TestGetElasticsearchVersionsFailedAuthNotRetried(t *testing.T) {
	for _, tc := range []struct {
		name       string
		httpStatus int
		cfg        RetrySettings
	}{
		{
			name:       "No retries are done for 401 failed request",
			httpStatus: http.StatusUnauthorized,
			cfg: RetrySettings{
				Enabled:       true,
				MaxRetries:    3,
				RetryOnStatus: []int{401},
			},
		},
		{
			name:       "No retries are done for 403 failed request",
			httpStatus: http.StatusForbidden,
			cfg: RetrySettings{
				Enabled:       true,
				MaxRetries:    3,
				RetryOnStatus: []int{403},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.httpStatus)
			}))
			t.Cleanup(server.Close)

			cfg := createDefaultConfig().(*Config)
			cfg.Endpoints = []string{server.URL}
			cfg.Retry = tc.cfg
			set := exportertest.NewNopSettings(metadata.Type)

			_ = getElasticsearchVersions(t.Context(), cfg, set, componenttest.NewNopHost())

			assert.Equalf(t, int32(1), calls.Load(), "%d must not be retried", tc.cfg.RetryOnStatus[0])
		})
	}
}

func TestSupportsExponentialHistograms(t *testing.T) {
	for _, tc := range []struct {
		name            string
		v               string
		expectedSupport bool
	}{
		{
			name:            "exponential histograms are supported",
			v:               "9.4.0",
			expectedSupport: true,
		},
		{
			name:            "exponential histograms are supported for 9.4.0 SNAPSHOT",
			v:               "9.4.0-SNAPSHOT",
			expectedSupport: true,
		},
		{
			name:            "exponential histograms are supported for SNAPSHOT",
			v:               "9.6.0-SNAPSHOT",
			expectedSupport: true,
		},
		{
			name:            "exponential histgorams are not supported",
			v:               "9.2.0",
			expectedSupport: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := semver.NewVersion(tc.v)
			require.NoError(t, err)
			actualSupported := supportsExponentialHistograms(v)
			assert.Equal(t, tc.expectedSupport, actualSupported)
		})
	}
}
