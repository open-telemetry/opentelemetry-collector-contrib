// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		context.Background(),
		cfg,
		componenttest.NewNopHost(),
		componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, couchdbClient)
	return couchdbClient
}

func TestNewCouchDBClient(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/non/existent",
		},
	}
	t.Run("Invalid config", func(t *testing.T) {
		couchdbClient, err := newCouchDBClient(
			context.Background(),
			&Config{
				ClientConfig: clientConfig,
			},
			componenttest.NewNopHost(),
			componenttest.NewNopTelemetrySettings())

		require.ErrorContains(t, err, "failed to create HTTP Client: ")
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
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/_stats/couchdb") {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "/invalid_endpoint") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "/invalid_body") {
			w.Header().Set("Content-Length", "1")
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	t.Run("invalid url request", func(t *testing.T) {
		url := ts.URL + " /space"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.Nil(t, result)
		require.ErrorContains(t, err, "invalid port ")
	})
	t.Run("invalid endpoint", func(t *testing.T) {
		url := ts.URL + "/invalid_endpoint"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.Nil(t, result)
		require.ErrorContains(t, err, "404 Not Found")
	})
	t.Run("invalid body", func(t *testing.T) {
		url := ts.URL + "/invalid_body"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.Nil(t, result)
		require.ErrorContains(t, err, "failed to read response body ")
	})
	t.Run("401 Unauthorized", func(t *testing.T) {
		url := ts.URL + "/_node/_local/_stats/couchdb"
		clientConfig := confighttp.NewDefaultClientConfig()
		clientConfig.Endpoint = url

		couchdbClient, err := newCouchDBClient(
			context.Background(),
			&Config{
				ClientConfig: clientConfig,
				Username:     "unauthorized",
				Password:     "unauthorized",
			},
			componenttest.NewNopHost(),
			componenttest.NewNopTelemetrySettings())
		require.NoError(t, err)
		require.NotNil(t, couchdbClient)

		result, err := couchdbClient.Get(url)
		require.Nil(t, result)
		require.ErrorContains(t, err, "401 Unauthorized")
	})
	t.Run("no error", func(t *testing.T) {
		url := ts.URL + "/_node/_local/_stats/couchdb"
		couchdbClient := defaultClient(t, url)

		result, err := couchdbClient.Get(url)
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

func TestGetNodeStats(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/invalid_json") {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"}`))
			assert.NoError(t, err)
			return
		}
		if strings.Contains(r.URL.Path, "/_stats/couchdb") {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"key":["value"]}`))
			assert.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	t.Run("invalid endpoint", func(t *testing.T) {
		couchdbClient := defaultClient(t, "invalid")

		actualStats, err := couchdbClient.GetStats("_local")
		require.Error(t, err)
		require.Nil(t, actualStats)
	})
	t.Run("invalid json", func(t *testing.T) {
		couchdbClient := defaultClient(t, ts.URL+"/invalid_json")

		actualStats, err := couchdbClient.GetStats("_local")
		require.Error(t, err)
		require.Nil(t, actualStats)
	})
	t.Run("no error", func(t *testing.T) {
		expectedStats := map[string]any{"key": []any{"value"}}
		couchdbClient := defaultClient(t, ts.URL)

		actualStats, err := couchdbClient.GetStats("_local")
		require.NoError(t, err)
		require.Equal(t, expectedStats, actualStats)
	})
}

func TestBuildReq(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	couchdbClient := couchDBClient{
		client: &http.Client{},
		cfg: &Config{
			ClientConfig: clientConfig,
			Username:     "otelu",
			Password:     "otelp",
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
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	couchdbClient := couchDBClient{
		client: &http.Client{},
		cfg: &Config{
			ClientConfig: clientConfig,
		},
		logger: zap.NewNop(),
	}

	_, err := couchdbClient.buildReq(" ")
	require.Error(t, err)
}
