// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crashreportextension

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.uber.org/zap"
)

func TestCrashReporter_Shutdown(t *testing.T) {
	reporter := &crashReportExtension{}
	err := reporter.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestCrashReporter_StartShutdownPanic(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://example.com"
	reporter := &crashReportExtension{
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	err := reporter.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	err = reporter.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestCrashReporter_InvalidClient(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Auth = &configauth.Authentication{AuthenticatorID: component.NewID("foo")}
	reporter := &crashReportExtension{
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	err := reporter.Start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestCrashReporter_handlePanic_invalidurl(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	reporter := &crashReportExtension{
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	err := reporter.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	reporter.handlePanic(errors.New("foo"))
}

type crashReader struct {
	statusCode int
	body       string
}

func (cr *crashReader) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	value, _ := io.ReadAll(req.Body)
	resp.WriteHeader(cr.statusCode)
	cr.body = string(value)
}

func TestCrashReporter_handlePanic_happyPath(t *testing.T) {
	reader := &crashReader{
		statusCode: 200,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/", reader)
	server := &http.Server{
		Handler: mux,
	}
	defer server.Close()
	go func() {
		_ = server.Serve(listener)
	}()
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "http://" + listener.Addr().String() + "/services/collector/raw"

	reporter := &crashReportExtension{
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	err = reporter.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer func() {
		_ = reporter.Shutdown(context.Background())
	}()

	reporter.handlePanic(errors.New("foo"))

	require.Contains(t, reader.body, "goroutine")
}
