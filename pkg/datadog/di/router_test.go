// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package di

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
)

func batch(records ...plog.LogRecord) plog.Logs {
	logs := plog.NewLogs()
	scope := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	scope.Scope().SetName("language-independent")
	for _, record := range records {
		record.CopyTo(scope.LogRecords().AppendEmpty())
	}
	return logs
}

func ordinaryRecord() plog.LogRecord {
	r := plog.NewLogRecord()
	r.Body().SetStr("application log")
	return r
}

func newTestRouter(t *testing.T, server *httptest.Server, ordinary func(context.Context, plog.Logs) error) *Router {
	t.Helper()
	r, err := New(componenttest.NewNopTelemetrySettings(), Options{
		Enabled: true, Key: "collector-key", Endpoint: server.URL + "/api/v2/debugger", HTTP: confighttp.NewDefaultClientConfig(),
	}, ordinary)
	require.NoError(t, err)
	r.client = server.Client()
	t.Cleanup(func() { require.NoError(t, r.Shutdown(t.Context())) })
	return r
}

func TestRoutingUsesOnlyEventName(t *testing.T) {
	input := batch(ordinaryRecord(), diRecord("dd.debugger.snapshot", `{"message":"capture"}`))
	input.ResourceLogs().At(0).ScopeLogs().At(0).Scope().SetName("com.datadoghq.dynamic_instrumentation")
	input.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutInt("datadog.di.schema_version", 1)
	di, ordinary := split(input)
	require.Equal(t, 1, di.LogRecordCount())
	require.Equal(t, 1, ordinary.LogRecordCount())
	require.Equal(t, "application log", ordinary.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
	require.Equal(t, 2, input.LogRecordCount())
	grouped := batch(diRecord("dd.debugger.snapshot", `{}`), diRecord("dd.debugger.log", `{}`))
	di, _ = split(grouped)
	require.Equal(t, 1, di.ResourceLogs().Len())
	require.Equal(t, 1, di.ResourceLogs().At(0).ScopeLogs().Len())
}

func TestRetryContainsOnlyFailedDestination(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	var ordinaryAttempts int
	router := newTestRouter(t, server, func(_ context.Context, input plog.Logs) error {
		ordinaryAttempts++
		require.Equal(t, 1, input.LogRecordCount())
		return nil
	})
	err := router.ConsumeLogs(t.Context(), batch(ordinaryRecord(), diRecord("dd.debugger.snapshot", `{"message":"capture"}`)))
	require.Error(t, err)
	require.False(t, consumererror.IsPermanent(err))
	var partial consumererror.Logs
	require.ErrorAs(t, err, &partial)
	require.Equal(t, 1, partial.Data().LogRecordCount())
	require.NoError(t, router.ConsumeLogs(t.Context(), partial.Data()))
	require.Equal(t, 1, ordinaryAttempts)
	require.Equal(t, int32(2), attempts.Load())
}

func TestMixedPermanentAndTransientFailures(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	defer server.Close()
	router := newTestRouter(t, server, func(context.Context, plog.Logs) error { return nil })
	err := router.ConsumeLogs(t.Context(), batch(diRecord("dd.debugger.snapshot", "invalid"), diRecord("dd.debugger.snapshot", `{}`)))
	require.Error(t, err)
	require.False(t, consumererror.IsPermanent(err))
	var partial consumererror.Logs
	require.ErrorAs(t, err, &partial)
	require.Equal(t, 1, partial.Data().LogRecordCount())
	v, _ := partial.Data().ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("datadog.di.payload")
	require.Equal(t, `{}`, v.Str())
}

func TestNativeOriginsAndResourceTags(t *testing.T) {
	var debugger, symbols atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") != "collector-key" || r.Header.Get("DD-REQUEST-ID") == "" {
			t.Error("missing Collector credentials or request ID")
		}
		_, _ = io.Copy(io.Discard, r.Body)
		switch r.Header.Get("DD-EVP-ORIGIN") {
		case "agent-debugger":
			debugger.Add(1)
			if r.URL.Query().Get("ddtags") != "env:test,version:one,host:application-host" {
				t.Error("missing debugger resource tags")
			}
		case "agent-symdb":
			symbols.Add(1)
			if r.Header.Get("X-Datadog-Additional-Tags") != "env:test,version:one,host:application-host" || r.URL.RawQuery != "" {
				t.Error("incorrect symbol tag placement")
			}
		default:
			t.Error("incorrect native origin")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	router := newTestRouter(t, server, func(context.Context, plog.Logs) error { return nil })
	input := batch(diRecord("dd.debugger.snapshot", `{"service":"checkout"}`), diRecord("dd.debugger.symbols", `{"event":{"runtimeId":"runtime-1","batchNum":1},"symbols":{"scopes":[]}}`))
	input.ResourceLogs().At(0).Resource().Attributes().PutStr("host.name", "application-host")
	input.ResourceLogs().At(0).Resource().Attributes().PutStr("service.name", "checkout")
	require.NoError(t, router.ConsumeLogs(t.Context(), input))
	require.Equal(t, int32(1), debugger.Load())
	require.Equal(t, int32(1), symbols.Load())
}

func TestDisabledUnknownAndConflictingRecordsAreNotOrdinaryLogs(t *testing.T) {
	for _, name := range []string{"disabled", "unknown", "conflict"} {
		t.Run(name, func(t *testing.T) {
			var received atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { received.Add(1); w.WriteHeader(http.StatusAccepted) }))
			defer server.Close()
			router := newTestRouter(t, server, func(context.Context, plog.Logs) error { t.Fatal("DI record reached ordinary log mapping"); return nil })
			r := diRecord("dd.debugger.snapshot", `{"service":"checkout"}`)
			if name == "disabled" {
				router.options.Enabled = false
			}
			if name == "unknown" {
				r.SetEventName("dd.debugger.future")
			}
			input := batch(r)
			if name == "conflict" {
				input.ResourceLogs().At(0).Resource().Attributes().PutStr("service.name", "other")
			}
			err := router.ConsumeLogs(t.Context(), input)
			require.Error(t, err)
			require.True(t, consumererror.IsPermanent(err))
			require.Zero(t, received.Load())
		})
	}
}

func TestOrdinaryPartialFailureRetainsOnlyRejectedRecords(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) }))
	defer server.Close()
	router := newTestRouter(t, server, func(_ context.Context, input plog.Logs) error {
		return consumererror.NewLogs(errors.New("retry ordinary"), input)
	})
	err := router.ConsumeLogs(t.Context(), batch(ordinaryRecord(), diRecord("dd.debugger.snapshot", `{}`)))
	var partial consumererror.Logs
	require.ErrorAs(t, err, &partial)
	require.Equal(t, 1, partial.Data().LogRecordCount())
	require.False(t, containsDI(partial.Data()))
	require.NotContains(t, strings.ToLower(err.Error()), "collector-key")
}

func TestNativeTagsRetainSDKValues(t *testing.T) {
	input := batch(diRecord("dd.debugger.snapshot", `{}`))
	rl := input.ResourceLogs().At(0)
	r := rl.ScopeLogs().At(0).LogRecords().At(0)
	r.Attributes().PutStr("datadog.di.tags", "env:test,custom:a,custom:b,flag")
	tags, err := nativeTags(rl, r)
	require.NoError(t, err)
	require.Equal(t, "env:test,custom:a,custom:b,flag", tags)
}
