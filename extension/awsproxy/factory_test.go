// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		ProxyConfig: proxy.Config{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "localhost:2000",
			},
		},
	}, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_Create(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		// Verify a signature was added, indicating the reverse proxy is doing its job.
		if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256") {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "No signature")
			return
		}
		w.Header().Set("Test", "Passed")
		fmt.Fprintln(w, "OK")
	}))
	defer backend.Close()

	cfg := createDefaultConfig().(*Config)
	address := testutil.GetAvailableLocalAddress(t)
	cfg.ProxyConfig.AWSEndpoint = backend.URL
	cfg.ProxyConfig.Endpoint = address
	cfg.ProxyConfig.Region = "us-east-2"

	// Simplest way to get SDK to use fake credentials
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	ctx := context.Background()
	cs := extensiontest.NewNopSettings(extensiontest.NopType)
	ext, err := createExtension(ctx, cs, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ext)

	host := &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			assert.NoError(t, event.Err())
		},
	}

	err = ext.Start(ctx, host)
	assert.NoError(t, err)

	var resp *http.Response
	require.Eventually(t, func() bool {
		resp, err = http.Post(
			"http://"+address+"/GetSamplingRules",
			"application/json",
			strings.NewReader(`{"NextToken": null}`))
		return err == nil
	}, 3*time.Second, 10*time.Millisecond)

	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Passed", resp.Header.Get("Test"))

	err = ext.Shutdown(ctx)
	assert.NoError(t, err)
}

var _ componentstatus.Reporter = (*nopHost)(nil)

type nopHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
