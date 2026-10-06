// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
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

func TestLogElasticsearchVersions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		handler         http.HandlerFunc
		expectConnected bool
		expectedVersion string
		expectedFlavor  string
	}{
		{
			name: "logs version and build flavor on success",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"version":{"number":"9.6.0","build_flavor":"default"}}`))
			},
			expectConnected: true,
			expectedVersion: "9.6.0",
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)

			core, observed := observer.New(zapcore.InfoLevel)
			set := exportertest.NewNopSettings(metadata.Type)
			set.Logger = zap.New(core)

			cfg := createDefaultConfig().(*Config)
			cfg.Endpoints = []string{srv.URL}

			logElasticsearchVersions(t.Context(), cfg, set, componenttest.NewNopHost())

			connected := observed.FilterMessage("Connected to Elasticsearch").All()
			if !tc.expectConnected {
				assert.Empty(t, connected)
				assert.NotEmpty(t, observed.FilterMessage("failed to fetch Elasticsearch info").All())
				return
			}

			require.Len(t, connected, 1)
			fields := connected[0].ContextMap()
			assert.Equal(t, tc.expectedVersion, fields["version"])
			assert.Equal(t, tc.expectedFlavor, fields["build_flavor"])
			assert.Equal(t, srv.URL, fields["endpoint"])
		})
	}
}
