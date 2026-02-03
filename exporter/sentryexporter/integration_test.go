// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter/internal/metadata"
)

func newTestServerConfig(ts *testServer) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.URL = ts.server.URL
	cfg.OrgSlug = "org"
	cfg.AuthToken = "token"
	cfg.AutoCreateProjects = true
	return cfg
}

func proxiedClient(serverURL string) *http.Client {
	u, _ := url.Parse(serverURL)
	hostPort := u.Host
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, hostPort)
			},
		},
	}
}

func tracesForProject(slug string) ptrace.Traces {
	td := generateLifecycleTestTraces()
	if td.ResourceSpans().Len() > 0 {
		td.ResourceSpans().At(0).Resource().Attributes().PutStr(DefaultAttributeForProject, slug)
	}
	return td
}

func logsForProject(slug string) plog.Logs {
	ld := generateLifecycleTestLogs()
	if ld.ResourceLogs().Len() > 0 {
		ld.ResourceLogs().At(0).Resource().Attributes().PutStr(DefaultAttributeForProject, slug)
	}
	return ld
}

func TestIntegration_OTLPRetryAfter429(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)
	testHTTPClient = proxiedClient(ts.server.URL)
	t.Cleanup(func() { testHTTPClient = nil })

	dsn := strings.Replace(ts.server.URL, "http://", "http://pubkey@", 1) + "/1"

	ts.queue(http.MethodGet, "/api/0/organizations/org/projects/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectInfo{
			{
				ID:   "seed",
				Slug: "seed",
				Team: teamInfo{Slug: "team"},
				Teams: []teamInfo{
					{Slug: "team"},
				},
			},
		}),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/proj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{
				ID:       "1",
				Public:   "pubkey",
				IsActive: true,
				DSN:      dsnField{Public: dsn},
			},
		}),
	})

	ts.queue(http.MethodPost, "/api/1/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusTooManyRequests,
		Headers: http.Header{
			"Retry-After": []string{"1"},
		},
		Body: []byte(`rate limited`),
	})
	ts.queue(http.MethodPost, "/api/1/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})

	cfg := newTestServerConfig(ts)
	set := exportertest.NewNopSettings(metadata.Type)
	state, err := newEndpointState(cfg, set)
	require.NoError(t, err)
	require.NoError(t, state.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = state.Shutdown(t.Context()) })

	td := tracesForProject("proj")

	err = state.pushTraceData(t.Context(), zap.NewNop(), td)
	require.Error(t, err)

	recs := ts.recorded(http.MethodPost, "/api/1/integration/otlp/v1/traces/")
	require.Len(t, recs, 1)
}

func TestIntegration_ProjectAutoCreateAndSend(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)
	testHTTPClient = proxiedClient(ts.server.URL)
	t.Cleanup(func() { testHTTPClient = nil })

	seedDSN := strings.Replace(ts.server.URL, "http://", "http://seed@", 1) + "/1"
	newProjDSN := strings.Replace(ts.server.URL, "http://", "http://pubkey@", 1) + "/2"

	ts.queue(http.MethodGet, "/api/0/organizations/org/projects/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectInfo{
			{
				ID:    "seed",
				Slug:  "seed",
				Team:  teamInfo{Slug: "team"},
				Teams: []teamInfo{{Slug: "team"}},
			},
		}),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/seed/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "seed", IsActive: true, DSN: dsnField{Public: seedDSN}},
		}),
	})

	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status: http.StatusNotFound,
		Body:   []byte(`not found`),
	})
	ts.queue(http.MethodPost, "/api/0/teams/org/team/projects/", testServerResponse{
		Status: http.StatusCreated,
		Body:   []byte(`{"id":"2","slug":"newproj","team":{"slug":"team"}}`),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "2", Public: "pubkey", IsActive: true, DSN: dsnField{Public: newProjDSN}},
		}),
	})
	ts.queue(http.MethodPost, "/api/2/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})

	cfg := newTestServerConfig(ts)
	set := exportertest.NewNopSettings(metadata.Type)
	state, err := newEndpointState(cfg, set)
	require.NoError(t, err)
	require.NoError(t, state.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = state.Shutdown(t.Context()) })

	td := tracesForProject("newproj")

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	require.Eventually(t, func() bool {
		return len(ts.recorded(http.MethodGet, "/api/0/projects/org/newproj/keys/")) >= 2
	}, 3*time.Second, 50*time.Millisecond)

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	recs := ts.recorded(http.MethodPost, "/api/2/integration/otlp/v1/traces/")
	require.Len(t, recs, 1)
}

func TestIntegration_ForbiddenWithProjectIDRejectedEvictsCacheAndRetries(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)
	testHTTPClient = proxiedClient(ts.server.URL)
	t.Cleanup(func() { testHTTPClient = nil })

	dsn := strings.Replace(ts.server.URL, "http://", "http://pubkey@", 1) + "/1"

	ts.queue(http.MethodGet, "/api/0/organizations/org/projects/", testServerResponse{
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
	ts.queue(http.MethodGet, "/api/0/projects/org/proj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "pubkey", IsActive: true, DSN: dsnField{Public: dsn}},
		}),
	})

	ts.queue(http.MethodPost, "/api/1/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusForbidden,
		Body:   []byte(`event submission rejected with_reason: ProjectId`),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/proj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "pubkey", IsActive: true, DSN: dsnField{Public: dsn}},
		}),
	})
	ts.queue(http.MethodPost, "/api/1/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})

	cfg := newTestServerConfig(ts)
	set := exportertest.NewNopSettings(metadata.Type)
	state, err := newEndpointState(cfg, set)
	require.NoError(t, err)
	require.NoError(t, state.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = state.Shutdown(t.Context()) })

	td := tracesForProject("proj")

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	recs := ts.recorded(http.MethodPost, "/api/1/integration/otlp/v1/traces/")
	require.Len(t, recs, 2)
}

func TestIntegration_ExistingProjectSend(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)
	testHTTPClient = proxiedClient(ts.server.URL)
	t.Cleanup(func() { testHTTPClient = nil })

	dsn := strings.Replace(ts.server.URL, "http://", "http://pubkey@", 1) + "/1"

	ts.queue(http.MethodGet, "/api/0/organizations/org/projects/", testServerResponse{
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
	ts.queue(http.MethodGet, "/api/0/projects/org/proj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "pubkey", IsActive: true, DSN: dsnField{Public: dsn}},
		}),
	})
	ts.queue(http.MethodPost, "/api/1/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})
	ts.queue(http.MethodPost, "/api/1/integration/otlp/v1/logs/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})

	cfg := newTestServerConfig(ts)
	set := exportertest.NewNopSettings(metadata.Type)
	state, err := newEndpointState(cfg, set)
	require.NoError(t, err)
	require.NoError(t, state.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = state.Shutdown(t.Context()) })

	td := tracesForProject("proj")
	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	ld := logsForProject("proj")
	require.NoError(t, state.pushLogData(t.Context(), zap.NewNop(), ld))

	traceRecs := ts.recorded(http.MethodPost, "/api/1/integration/otlp/v1/traces/")
	logRecs := ts.recorded(http.MethodPost, "/api/1/integration/otlp/v1/logs/")
	require.Len(t, traceRecs, 1)
	require.Len(t, logRecs, 1)
}

func TestIntegration_ProjectCreateRateLimitedThenSucceeds(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)
	testHTTPClient = proxiedClient(ts.server.URL)
	t.Cleanup(func() { testHTTPClient = nil })

	seedDSN := strings.Replace(ts.server.URL, "http://", "http://seed@", 1) + "/1"
	newDSN := strings.Replace(ts.server.URL, "http://", "http://pubkey@", 1) + "/2"

	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status: http.StatusNotFound,
		Body:   []byte(`not found`),
	})

	ts.queue(http.MethodGet, "/api/0/organizations/org/projects/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectInfo{
			{
				ID:    "seed",
				Slug:  "seed",
				Team:  teamInfo{Slug: "team"},
				Teams: []teamInfo{{Slug: "team"}},
			},
		}),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/seed/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "seed", IsActive: true, DSN: dsnField{Public: seedDSN}},
		}),
	})

	ts.queue(http.MethodPost, "/api/0/teams/org/team/projects/", testServerResponse{
		Status:  http.StatusTooManyRequests,
		Headers: http.Header{"X-Sentry-Rate-Limit-Reset": []string{"0"}},
		Body:    []byte(`rate limited`),
	})
	ts.queue(http.MethodPost, "/api/0/teams/org/team/projects/", testServerResponse{
		Status: http.StatusCreated,
		Body:   []byte(`{"id":"2","slug":"newproj","team":{"slug":"team"}}`),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "2", Public: "pubkey", IsActive: true, DSN: dsnField{Public: newDSN}},
		}),
	})
	ts.queue(http.MethodPost, "/api/2/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})

	cfg := newTestServerConfig(ts)
	set := exportertest.NewNopSettings(metadata.Type)
	state, err := newEndpointState(cfg, set)
	require.NoError(t, err)
	require.NoError(t, state.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = state.Shutdown(t.Context()) })

	td := tracesForProject("newproj")

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	require.Eventually(t, func() bool {
		return len(ts.recorded(http.MethodPost, "/api/0/teams/org/team/projects/")) >= 2
	}, 2*time.Second, 50*time.Millisecond)

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	recs := ts.recorded(http.MethodPost, "/api/2/integration/otlp/v1/traces/")
	require.Len(t, recs, 1)
}

func TestIntegration_EndpointFetchRateLimitedThenSucceeds(t *testing.T) {
	ts := newTestServer()
	t.Cleanup(ts.close)
	testHTTPClient = proxiedClient(ts.server.URL)
	t.Cleanup(func() { testHTTPClient = nil })

	seedDSN := strings.Replace(ts.server.URL, "http://", "http://seed@", 1) + "/1"
	newDSN := strings.Replace(ts.server.URL, "http://", "http://pubkey@", 1) + "/2"

	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status: http.StatusNotFound,
		Body:   []byte(`not found`),
	})

	ts.queue(http.MethodGet, "/api/0/organizations/org/projects/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectInfo{
			{
				ID:    "seed",
				Slug:  "seed",
				Team:  teamInfo{Slug: "team"},
				Teams: []teamInfo{{Slug: "team"}},
			},
		}),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/seed/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "1", Public: "seed", IsActive: true, DSN: dsnField{Public: seedDSN}},
		}),
	})

	ts.queue(http.MethodPost, "/api/0/teams/org/team/projects/", testServerResponse{
		Status: http.StatusCreated,
		Body:   []byte(`{"id":"2","slug":"newproj","team":{"slug":"team"}}`),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status:  http.StatusTooManyRequests,
		Headers: http.Header{"X-Sentry-Rate-Limit-Reset": []string{"0"}},
		Body:    []byte(`rate limited`),
	})
	ts.queue(http.MethodGet, "/api/0/projects/org/newproj/keys/", testServerResponse{
		Status: http.StatusOK,
		Body: mustJSON([]projectKey{
			{ID: "2", Public: "pubkey", IsActive: true, DSN: dsnField{Public: newDSN}},
		}),
	})
	ts.queue(http.MethodPost, "/api/2/integration/otlp/v1/traces/", testServerResponse{
		Status: http.StatusOK,
		Body:   []byte(`{}`),
	})

	cfg := newTestServerConfig(ts)
	set := exportertest.NewNopSettings(metadata.Type)
	state, err := newEndpointState(cfg, set)
	require.NoError(t, err)
	require.NoError(t, state.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = state.Shutdown(t.Context()) })

	td := tracesForProject("newproj")

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	require.Eventually(t, func() bool {
		return len(ts.recorded(http.MethodGet, "/api/0/projects/org/newproj/keys/")) >= 2
	}, 2*time.Second, 50*time.Millisecond)

	require.NoError(t, state.pushTraceData(t.Context(), zap.NewNop(), td))

	recs := ts.recorded(http.MethodPost, "/api/2/integration/otlp/v1/traces/")
	require.Len(t, recs, 1)
}
