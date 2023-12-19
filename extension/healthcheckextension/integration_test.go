// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func Test_SimpleHealthCheck(t *testing.T) {
	f := NewFactory()
	port := testbed.GetAvailablePort(t)
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = fmt.Sprintf("localhost:%d", port)
	e, err := f.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, e.Shutdown(context.Background()))
	})
	resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, "503 Service Unavailable", resp.Status)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"status":"Server not available","upSince":"0001-01-01T00:00:00Z","uptime":""}`, buf.String())
	err = e.(*healthCheckExtension).Ready()
	require.NoError(t, err)
	resp, err = http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d/", port))
	require.NoError(t, err)
	assert.Equal(t, "200 OK", resp.Status)
	buf.Reset()
	_, err = io.Copy(&buf, resp.Body)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `{"status":"Server available","upSince":"`)
}
