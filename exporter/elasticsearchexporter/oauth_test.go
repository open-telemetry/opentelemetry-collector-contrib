// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
)

type mockComponentHost struct {
	extensions map[component.ID]component.Component
}

func (mockcomponenthost *mockComponentHost) ReportFatalError(_ error) {
}

func (mockcomponenthost *mockComponentHost) GetFactory(_ component.Kind, _ component.Type) component.Factory {
	return nil
}

func (mockcomponenthost *mockComponentHost) GetExtensions() map[component.ID]component.Component {
	return mockcomponenthost.extensions
}

func (mockcomponenthost *mockComponentHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return nil
}

func TestExpoprtOAuth(t *testing.T) {
	t.Run("auth success", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})
		tokenEndpoint := "/oauth2/token" //nolint:gosec
		authServer := newOauthMockServer(t, tokenEndpoint, true)
		id, host := createComponentHost(t, authServer.URL+tokenEndpoint)

		exporter := newTestExporterWithHost(t, zaptest.NewLogger(t), server.URL, host, func(c *Config) {
			c.Authentication = AuthenticationSettings{
				OAuth: &configauth.Authentication{
					AuthenticatorID: id,
				},
			}

		})
		mustSend(t, exporter, `{"message": "test1"}`)
		mustSend(t, exporter, `{"message": "test2"}`)
		rec.WaitItems(2)
	})

	t.Run("auth failure", func(t *testing.T) {
		rec := newBulkRecorder()
		server := newESTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
			rec.Record(docs)
			return itemsAllOK(docs)
		})
		tokenEndpoint := "/oauth2/token" //nolint:gosec
		authServer := newOauthMockServer(t, tokenEndpoint, false)
		id, host := createComponentHost(t, authServer.URL+tokenEndpoint)

		observedZapCore, observedLogs := observer.New(zap.InfoLevel)
		core := zapcore.NewTee(observedZapCore, zaptest.NewLogger(t).Core())
		observedLogger := zap.New(core)
		exporter := newTestExporterWithHost(t, observedLogger, server.URL, host, func(c *Config) {
			c.Authentication = AuthenticationSettings{
				OAuth: &configauth.Authentication{
					AuthenticatorID: id,
				},
			}
		})
		mustSend(t, exporter, `{"message": "test1"}`)
		time.Sleep(time.Millisecond * 100)
		assert.Equal(t, 0, rec.NumItems())
		filtered := observedLogs.FilterMessageSnippet("failed to get security token from token endpoin")
		assert.NotEqual(t, 0, filtered.Len())
	})
}

func createComponentHost(t *testing.T, tokenURL string) (component.ID, *mockComponentHost) {
	factory := oauth2clientauthextension.NewFactory()

	id := component.NewID(component.Type("oauth2client"))
	config := factory.CreateDefaultConfig().(*oauth2clientauthextension.Config)
	config.ClientID = "fakeclientid"
	config.ClientSecret = "fakeclientsecret"
	config.TokenURL = tokenURL
	require.NoError(t, config.Validate())

	extension, err := factory.CreateExtension(context.Background(), extension.CreateSettings{ID: id}, config)
	require.NoError(t, err)

	host := &mockComponentHost{
		extensions: map[component.ID]component.Component{
			id: extension,
		},
	}
	return id, host
}

func newTestExporterWithHost(t *testing.T, logger *zap.Logger, url string, host component.Host, fns ...func(*Config)) *elasticsearchLogsExporter {

	exporter, err := newLogsExporter(logger, withTestExporterConfig(fns...)(url))
	require.NoError(t, err)

	err = exporter.start(context.Background(), host)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.TODO()))
	})
	return exporter
}

func newOauthMockServer(t *testing.T, path string, authSuccess bool) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if authSuccess {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "MTQ0NjJkZmQ5OTM2NDE1ZTZjNGZmZjI3",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "IwOGYzYTlmM2YxOTQ5MGE3YmNmMDFkNTVk",
				"scope":         "create",
			})
		} else {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":             "some error",
				"error_description": "error description",
				"error_uri":         "error uri",
			})
		}
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
