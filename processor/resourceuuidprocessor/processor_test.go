package resourceuuidprocessor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const graphJSON = `{"meta":{},"nodes":[
 {"moid":"cl_uid-1","uuid":"uuid-1","kind":"Pod"},
 {"moid":"cl_uid-2","uuid":"","kind":"Pod"},
 {"moid":"cl_node1","uuid":"node-uuid","kind":"Node"}],"edges":[]}`

func logsFor(uid string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	if uid != "" {
		rl.Resource().Attributes().PutStr("k8s.pod.uid", uid)
	}
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("hello")
	return ld
}

func uuidOf(ld plog.Logs) (string, bool) {
	v, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get("k8s.pod.uuid")
	return v.Str(), ok
}

func newTestProcessor(t *testing.T, url string, mutate func(*Config)) *resourceUUIDProcessor {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = url
	if mutate != nil {
		mutate(cfg)
	}
	require.NoError(t, cfg.Validate())
	return newResourceUUIDProcessor(zap.NewNop(), cfg)
}

func TestFetchParsesPodsOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(graphJSON))
	}))
	defer srv.Close()

	pods, unresolved, err := newPodClient(srv.URL, time.Second).fetch(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]resourceMetadata{"uid-1": {uuid: "uuid-1"}}, pods)
	require.Equal(t, 1, unresolved)
}

func TestFetchErrorOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, err := newPodClient(srv.URL, time.Second).fetch(context.Background())
	require.Error(t, err)
}

func TestProcessLogsAddsUUIDOnlyForResolvedPods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(graphJSON))
	}))
	defer srv.Close()

	p := newTestProcessor(t, srv.URL, nil)
	p.refresh(context.Background())

	out, err := p.processLogs(context.Background(), logsFor("uid-1"))
	require.NoError(t, err)
	uuid, ok := uuidOf(out)
	require.True(t, ok)
	require.Equal(t, "uuid-1", uuid)
	require.False(t, p.missed.Load())

	out, _ = p.processLogs(context.Background(), logsFor("uid-2"))
	_, ok = uuidOf(out)
	require.False(t, ok, "a pod without a uuid passes through untouched")
	require.True(t, p.missed.Load())

	out, _ = p.processLogs(context.Background(), logsFor(""))
	_, ok = uuidOf(out)
	require.False(t, ok)
	require.Equal(t, 1, out.LogRecordCount())
}

func TestFailedRefreshKeepsCache(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(graphJSON))
	}))
	defer srv.Close()

	p := newTestProcessor(t, srv.URL, nil)
	p.refresh(context.Background())
	fail.Store(true)
	p.refresh(context.Background())

	out, _ := p.processLogs(context.Background(), logsFor("uid-1"))
	uuid, ok := uuidOf(out)
	require.True(t, ok)
	require.Equal(t, "uuid-1", uuid)
}

func TestCacheEntryExpiresAfterTTL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(graphJSON))
	}))
	defer srv.Close()

	p := newTestProcessor(t, srv.URL, func(c *Config) {
		c.CacheTTL = 50 * time.Millisecond
		c.RetryInterval = 10 * time.Millisecond
	})
	p.refresh(context.Background())
	time.Sleep(100 * time.Millisecond)

	out, _ := p.processLogs(context.Background(), logsFor("uid-1"))
	_, ok := uuidOf(out)
	require.False(t, ok)
}

func TestNewPodResolvesOnRetryTick(t *testing.T) {
	var resolved atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if resolved.Load() {
			_, _ = w.Write([]byte(`{"nodes":[{"moid":"cl_uid-2","uuid":"uuid-2","kind":"Pod"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"nodes":[{"moid":"cl_uid-2","uuid":"","kind":"Pod"}]}`))
	}))
	defer srv.Close()

	p := newTestProcessor(t, srv.URL, func(c *Config) {
		c.RefreshInterval = time.Hour
		c.RetryInterval = 20 * time.Millisecond
	})
	require.NoError(t, p.start(context.Background(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, p.shutdown(context.Background())) }()

	time.Sleep(50 * time.Millisecond)
	out, _ := p.processLogs(context.Background(), logsFor("uid-2"))
	_, ok := uuidOf(out)
	require.False(t, ok)

	resolved.Store(true)
	require.Eventually(t, func() bool {
		out, _ := p.processLogs(context.Background(), logsFor("uid-2"))
		uuid, ok := uuidOf(out)
		return ok && uuid == "uuid-2"
	}, 2*time.Second, 10*time.Millisecond)
}

func TestRepeatedFailuresEscalateToErrorAndRecoveryIsLogged(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(graphJSON))
	}))
	defer srv.Close()

	core, logs := observer.New(zapcore.InfoLevel)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = srv.URL
	p := newResourceUUIDProcessor(zap.New(core), cfg)

	for i := 0; i < failuresBeforeError; i++ {
		p.refresh(context.Background())
	}
	require.Equal(t, failuresBeforeError-1, logs.FilterLevelExact(zapcore.WarnLevel).Len())
	require.Equal(t, 1, logs.FilterLevelExact(zapcore.ErrorLevel).Len())

	fail.Store(false)
	p.refresh(context.Background())
	require.Equal(t, 1, logs.FilterMessage("pod uuid endpoint recovered").Len())
	require.Equal(t, 1, logs.FilterMessage("refreshed pod uuids").Len())
}

func TestUsedEntrySurvivesAndAbsentPodIsKept(t *testing.T) {
	var gone atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if gone.Load() {
			_, _ = w.Write([]byte(`{"nodes":[]}`))
			return
		}
		_, _ = w.Write([]byte(graphJSON))
	}))
	defer srv.Close()

	p := newTestProcessor(t, srv.URL, func(c *Config) {
		c.CacheTTL = 200 * time.Millisecond
		c.RetryInterval = 10 * time.Millisecond
		c.RefreshInterval = 10 * time.Millisecond
	})
	p.refresh(context.Background())
	gone.Store(true)

	for i := 0; i < 4; i++ {
		time.Sleep(100 * time.Millisecond)
		p.refresh(context.Background())
		out, _ := p.processLogs(context.Background(), logsFor("uid-1"))
		_, ok := uuidOf(out)
		require.True(t, ok, "entry in use must stay cached after %d refreshes", i+1)
	}

	time.Sleep(300 * time.Millisecond)
	out, _ := p.processLogs(context.Background(), logsFor("uid-1"))
	_, ok := uuidOf(out)
	require.False(t, ok, "entry unused for longer than cache_ttl must be dropped")
}
