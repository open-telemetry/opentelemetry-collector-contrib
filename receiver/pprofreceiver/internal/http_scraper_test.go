// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestHttpScraper(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := http.Server{
		Handler:           pprof.Handler("goroutine"),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err2 := server.Serve(listener); !errors.Is(err2, http.ErrServerClosed) {
			panic(err2)
		}
	}()
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		require.NoError(t, server.Shutdown(ctx))
	})

	s := HTTPClientScraper{
		ClientConfig: confighttp.NewDefaultClientConfig(),
	}
	s.ClientConfig.Endpoint = fmt.Sprintf("http://%s/debug/pprof/", listener.Addr().String())
	err = s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()
	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())
}
