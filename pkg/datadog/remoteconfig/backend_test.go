// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

package remoteconfig

import (
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	configmock "github.com/DataDog/datadog-agent/pkg/config/mock"
	pbgo "github.com/DataDog/datadog-agent/pkg/proto/pbgo/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"google.golang.org/protobuf/proto"
)

func TestCollectorBackendTrustAndRedirectPolicy(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	t.Cleanup(target.Close)
	var redirect atomic.Bool
	want := &pbgo.OrgDataResponse{Uuid: "test-org"}
	body, err := proto.Marshal(want)
	require.NoError(t, err)
	backend := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, []string{"collector-secret"}, r.Header.Values("DD-API-KEY"))
		if redirect.Load() {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(backend.Close)
	e := testExtension(t, &fakeService{})
	u, err := url.Parse(backend.URL)
	require.NoError(t, err)
	client, closeClient, err := e.newAPIClient(t.Context(), componenttest.NewNopHost(), configmock.New(t), u)
	require.NoError(t, err)
	_, err = client.FetchOrgData(t.Context())
	require.Error(t, err, "the backend's private CA must not be trusted implicitly")
	closeClient()

	ca := filepath.Join(t.TempDir(), "backend-ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: backend.Certificate().Raw}), 0o600))
	e.config.Backend.TLS.CAFile = ca
	client, closeClient, err = e.newAPIClient(t.Context(), componenttest.NewNopHost(), configmock.New(t), u)
	require.NoError(t, err)
	t.Cleanup(closeClient)
	got, err := client.FetchOrgData(t.Context())
	require.NoError(t, err)
	require.True(t, proto.Equal(want, got))
	redirect.Store(true)
	_, err = client.FetchOrgData(t.Context())
	require.Error(t, err)
	require.Zero(t, redirected.Load(), "Collector authentication must stay at the configured origin")
}

func TestCollectorBackendReleasesIdleConnection(t *testing.T) {
	idle := make(chan net.Conn, 1)
	closed := make(chan net.Conn, 1)
	body, err := proto.Marshal(&pbgo.OrgDataResponse{Uuid: "test-org"})
	require.NoError(t, err)
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	backend.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateIdle:
			idle <- conn
		case http.StateClosed:
			closed <- conn
		}
	}
	backend.StartTLS()
	t.Cleanup(backend.Close)
	e := testExtension(t, &fakeService{})
	ca := filepath.Join(t.TempDir(), "backend-ca.pem")
	require.NoError(t, os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: backend.Certificate().Raw}), 0o600))
	e.config.Backend.TLS.CAFile = ca
	u, err := url.Parse(backend.URL)
	require.NoError(t, err)
	client, closeClient, err := e.newAPIClient(t.Context(), componenttest.NewNopHost(), configmock.New(t), u)
	require.NoError(t, err)
	t.Cleanup(closeClient)
	_, err = client.FetchOrgData(t.Context())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var connection net.Conn
	select {
	case connection = <-idle:
	case <-ctx.Done():
		t.Fatal("successful request did not leave an idle connection")
	}
	select {
	case <-closed:
		t.Fatal("connection closed before cleanup was invoked")
	default:
	}
	closeClient()
	select {
	case got := <-closed:
		require.Same(t, connection, got)
	case <-ctx.Done():
		t.Fatal("OTel instrumentation did not preserve pool cleanup")
	}
}
