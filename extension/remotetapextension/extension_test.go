// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
)

func Test_handleTaps(t *testing.T) {
	r := &remoteObserverExtension{
		config: &Config{Taps: []TapInfo{{
			Name:     "foo",
			Endpoint: "localhost:1234",
		}}},
	}
	req := httptest.NewRequest("GET", "/taps", bytes.NewReader([]byte{}))
	resp := httptest.NewRecorder()
	r.handleTaps(resp, req)
	require.Equal(t, 200, resp.Code)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `[{"name":"foo","endpoint":"localhost:1234"}]`, string(b))
}

func Test_ServeHTTP(t *testing.T) {
	r := &remoteObserverExtension{
		config: &Config{
			HTTPServerSettings: confighttp.HTTPServerSettings{
				Endpoint: defaultEndpoint,
			},
			Taps: []TapInfo{{
				Name:     "foo",
				Endpoint: "localhost:1234",
			}}},
	}
	err := r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, r.Shutdown(context.Background()))
	})
	time.Sleep(1 * time.Second)
	client := http.DefaultClient
	resp, err := client.Get(fmt.Sprintf("http://%s/taps", defaultEndpoint))
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `[{"name":"foo","endpoint":"localhost:1234"}]`, string(b))

	resp, err = client.Get(fmt.Sprintf("http://%s/index.js", defaultEndpoint))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}
