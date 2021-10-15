// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsxrayproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
		ProxyConfig: proxy.Config{
			TCPAddr: confignet.TCPAddr{
				Endpoint: defaultEndpoint,
			},
		},
	}, cfg)

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
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
	os.Setenv("AWS_ACCESS_KEY_ID", "fakeAccessKeyID")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "fakeSecretAccessKey")

	ctx := context.Background()
	ext, err := createExtension(ctx, componenttest.NewNopExtensionCreateSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ext)

	mh := newAssertNoErrorHost(t)
	err = ext.Start(ctx, mh)
	assert.NoError(t, err)

	resp, err := http.Post(
		"http://"+address+"/GetSamplingRules",
		"application/json",
		strings.NewReader(`{"NextToken": null}`))

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
