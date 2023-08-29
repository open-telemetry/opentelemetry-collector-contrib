// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
)

func defaultClient(t *testing.T, endpoint string) client {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = endpoint

	couchdbClient, err := newCouchDBClient(
		cfg,
		componenttest.NewNopHost(),
		componenttest.NewNopTelemetrySettings())
	require.Nil(t, err)
	require.NotNil(t, couchdbClient)
	return couchdbClient
}

func TestNewCouchDBClient(t *testing.T) {
	t.Run("Invalid config", func(t *testing.T) {
		couchdbClient, err := newCouchDBClient(
			&Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "/non/existent",
						},
					},
				}},
			componenttest.NewNopHost(),
			componenttest.NewNopTelemetrySettings())

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create HTTP Client: ")
		require.Nil(t, couchdbClient)
	})
	t.Run("no error", func(t *testing.T) {
		client := defaultClient(t, defaultEndpoint)
		require.NotNil(t, client)
	})
}

func TestGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		if u == "unauthorized" || p == "unauthorized" {
			w.WriteHeader(401)
			return
		}
		if strings.Contains(r.URL.Path, "/_stats/couchdb") {
			w.WriteHeader(200)
			return
		}
		if strings.Contains(r.URL.Path, "/invalid_endpoint") {
			w.WriteHeader(404)
			return
		}
		if strings.Contains(r.URL.Path, "/invalid_body") {
			w.Header().Set("Content-Length", "1")
			return
		}

		w.WriteHeader(404)
	}))
	defer ts.Close()

	t.Run("invalid url request", func(t *testing.T) {
		url := ts.URL + " /space"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.NotNil(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "invalid port ")
	})
	t.Run("invalid endpoint", func(t *testing.T) {
		url := ts.URL + "/invalid_endpoint"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.NotNil(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "404 Not Found")
	})
	t.Run("invalid body", func(t *testing.T) {
		url := ts.URL + "/invalid_body"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.NotNil(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to read response body ")
	})
	t.Run("401 Unauthorized", func(t *testing.T) {
		url := ts.URL + "/_node/_local/_stats/couchdb"
		couchdbClient, err := newCouchDBClient(
			&Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: url,
				},
				Username: "unauthorized",
				Password: "unauthorized",
			},
			componenttest.NewNopHost(),
			componenttest.NewNopTelemetrySettings())
		require.Nil(t, err)
		require.NotNil(t, couchdbClient)

		result, err := couchdbClient.Get(url)
		require.Nil(t, result)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "401 Unauthorized")
	})
	t.Run("no error", func(t *testing.T) {
		url := ts.URL + "/_node/_local/_stats/couchdb"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.Nil(t, err)
		require.NotNil(t, result)
	})
}

func TestGetNodeStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if strings.Contains(r.URL.Path, "/invalid_json") {
			w.WriteHeader(200)
			_, err := w.Write([]byte(`{"}`))
			require.NoError(t, err)
			return
		}
		if strings.Contains(r.URL.Path, "/_stats/couchdb") {
			w.WriteHeader(200)
			_, err := w.Write([]byte(`{"key":["value"]}`))
			require.NoError(t, err)
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()

	t.Run("invalid endpoint", func(t *testing.T) {
		couchdbClient := defaultClient(t, "invalid")

		actualStats, err := couchdbClient.GetStats("_local")
		require.NotNil(t, err)
		require.Nil(t, actualStats)
	})
	t.Run("invalid json", func(t *testing.T) {
		couchdbClient := defaultClient(t, ts.URL+"/invalid_json")

		actualStats, err := couchdbClient.GetStats("_local")
		require.NotNil(t, err)
		require.Nil(t, actualStats)
	})
	t.Run("no error", func(t *testing.T) {
		expectedStats := map[string]interface{}{"key": []interface{}{"value"}}
		couchdbClient := defaultClient(t, ts.URL)

		actualStats, err := couchdbClient.GetStats("_local")
		require.Nil(t, err)
		require.EqualValues(t, expectedStats, actualStats)
	})
}

func TestBuildReq(t *testing.T) {
	couchdbClient := couchDBClient{
		client: &http.Client{},
		cfg: &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: defaultEndpoint,
			},
			Username: "otelu",
			Password: "otelp",
		},
		logger: zap.NewNop(),
	}

	path := "/_node/_local/_stats/couchdb"
	req, err := couchdbClient.buildReq(path)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, "application/json", req.Header["Content-Type"][0])
	require.Equal(t, []string{"Basic b3RlbHU6b3RlbHA="}, req.Header["Authorization"])
	require.Equal(t, defaultEndpoint+path, req.URL.String())
}

func TestBuildBadReq(t *testing.T) {
	couchdbClient := couchDBClient{
		client: &http.Client{},
		cfg: &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: defaultEndpoint,
			},
		},
		logger: zap.NewNop(),
	}

	_, err := couchdbClient.buildReq(" ")
	require.Error(t, err)
}
