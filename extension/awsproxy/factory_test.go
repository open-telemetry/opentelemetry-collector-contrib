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
			TCPAddr: confignet.TCPAddr{
				Endpoint: defaultEndpoint,
			},
		},
	}, cfg)

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateExtension(t *testing.T) {
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
	cfg.ProxyConfig.TCPAddr.Endpoint = address
	cfg.ProxyConfig.Region = "us-east-2"

	// Simplest way to get SDK to use fake credentials
	t.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	ctx := context.Background()
	ext, err := createExtension(ctx, extensiontest.NewNopCreateSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ext)

	mh := newAssertNoErrorHost(t)
	err = ext.Start(ctx, mh)
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

// assertNoErrorHost implements a component.Host that asserts that there were no errors.
type assertNoErrorHost struct {
	component.Host
	*testing.T
}

// newAssertNoErrorHost returns a new instance of assertNoErrorHost.
func newAssertNoErrorHost(t *testing.T) component.Host {
	return &assertNoErrorHost{
		Host: componenttest.NewNopHost(),
		T:    t,
	}
}

func (aneh *assertNoErrorHost) ReportFatalError(err error) {
	assert.NoError(aneh, err)
}
