// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, "sentry", factory.Type().String())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)
	err := cfg.(*Config).Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "'url' must be configured")
}

func TestCreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		URL:       "https://sentry.io",
		OrgSlug:   "test-org",
		AuthToken: "test-token",
	}

	set := exportertest.NewNopSettings(factory.Type())
	exp, err := createTracesExporter(t.Context(), set, cfg)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		URL:       "https://sentry.io",
		OrgSlug:   "test-org",
		AuthToken: "test-token",
	}

	set := exportertest.NewNopSettings(factory.Type())
	exp, err := createLogsExporter(t.Context(), set, cfg)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateExporterWithInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{} // Invalid: no dynamic config

	set := exportertest.NewNopSettings(factory.Type())
	_, err := createTracesExporter(t.Context(), set, cfg)

	assert.Error(t, err)
}

func TestSharedComponentSingleton(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)

	dsn := dsnForServer("pubkey", ts)
	ts.queue(http.MethodGet, "/api/0/organizations/test-org/projects/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectInfo{
			{
				ID:    "1",
				Slug:  "proj",
				Team:  teamInfo{Slug: "team"},
				Teams: []teamInfo{{Slug: "team"}},
			},
		}),
	})
	ts.queue(http.MethodGet, "/api/0/projects/test-org/proj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "pubkey", IsActive: true, DSN: dsnField{Public: dsn}},
		}),
	})

	cfg := &Config{
		URL:       ts.server.URL,
		OrgSlug:   "test-org",
		AuthToken: "test-token",
	}
	cfg.ForceAttemptHTTP2 = false

	factory := NewFactory()
	set := exportertest.NewNopSettings(factory.Type())

	tracesExp, err := factory.CreateTraces(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, tracesExp)

	logsExp, err := factory.CreateLogs(t.Context(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, logsExp)

	sc1, st1, err := getOrCreateEndpointState(cfg, set)
	require.NoError(t, err)
	sc2, st2, err := getOrCreateEndpointState(cfg, set)
	require.NoError(t, err)

	assert.Same(t, sc1, sc2, "SharedComponents should be the same instance")
	assert.Same(t, st1, st2, "Unwrapped states should be the same instance")

	err = tracesExp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	err = logsExp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	err = tracesExp.Shutdown(t.Context())
	require.NoError(t, err)
	err = logsExp.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestSharedComponentDifferentConfigs(t *testing.T) {
	cfg1 := &Config{
		URL:       "https://sentry.io",
		OrgSlug:   "org1",
		AuthToken: "test-token",
	}

	cfg2 := &Config{
		URL:       "https://sentry.io",
		OrgSlug:   "org2",
		AuthToken: "test-token",
	}

	factory := NewFactory()
	set := exportertest.NewNopSettings(factory.Type())

	exp1, err := factory.CreateTraces(t.Context(), set, cfg1)
	require.NoError(t, err)

	exp2, err := factory.CreateTraces(t.Context(), set, cfg2)
	require.NoError(t, err)

	_, st1, err := getOrCreateEndpointState(cfg1, set)
	require.NoError(t, err)
	_, st2, err := getOrCreateEndpointState(cfg2, set)
	require.NoError(t, err)

	assert.NotSame(t, st1, st2, "Different configs should create different states")

	err = exp1.Shutdown(t.Context())
	require.NoError(t, err)

	err = exp2.Shutdown(t.Context())
	require.NoError(t, err)
}
