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

package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jaegertracing/jaeger/thrift-gen/sampling"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestMissingClientConfigManagerHTTP(t *testing.T) {
	s, err := NewHTTP(componenttest.NewNopTelemetrySettings(), confighttp.HTTPServerSettings{}, nil)
	assert.Equal(t, errMissingStrategyStore, err)
	assert.Nil(t, s)
}

func TestStartAndStopHTTP(t *testing.T) {
	// prepare
	srvSettings := confighttp.HTTPServerSettings{
		Endpoint: ":0",
	}
	s, err := NewHTTP(componenttest.NewNopTelemetrySettings(), srvSettings, &mockCfgMgr{})
	require.NoError(t, err)
	require.NotNil(t, s)

	// test
	assert.NoError(t, s.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, s.Shutdown(context.Background()))
}

func TestEndpointsAreWired(t *testing.T) {
	testCases := []struct {
		desc     string
		endpoint string
	}{
		{
			desc:     "legacy",
			endpoint: "/",
		},
		{
			desc:     "new",
			endpoint: "/sampling",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// prepare
			s, err := NewHTTP(componenttest.NewNopTelemetrySettings(), confighttp.HTTPServerSettings{}, &mockCfgMgr{})
			require.NoError(t, err)
			require.NotNil(t, s)

			srv := httptest.NewServer(s.mux)
			defer func() {
				srv.Close()
			}()

			// test
			resp, err := srv.Client().Get(fmt.Sprintf("%s%s?service=foo", srv.URL, tC.endpoint))
			require.NoError(t, err)

			// verify
			samplingStrategiesBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()

			body := string(samplingStrategiesBytes)
			assert.Equal(t, `{"strategyType":"PROBABILISTIC"}`, body)
		})
	}
}

func TestServiceNameIsRequired(t *testing.T) {
	// prepare
	s, err := NewHTTP(componenttest.NewNopTelemetrySettings(), confighttp.HTTPServerSettings{}, &mockCfgMgr{})
	require.NoError(t, err)
	require.NotNil(t, s)

	rw := httptest.NewRecorder()
	req := &http.Request{
		URL: &url.URL{},
	}

	// test
	s.samplingStrategyHandler(rw, req)

	// verify
	body, _ := io.ReadAll(rw.Body)
	assert.Contains(t, string(body), "'service' parameter must be provided")
}

func TestErrorFromClientConfigManager(t *testing.T) {
	s, err := NewHTTP(componenttest.NewNopTelemetrySettings(), confighttp.HTTPServerSettings{}, &mockCfgMgr{})
	require.NoError(t, err)
	require.NotNil(t, s)

	s.strategyStore = &mockCfgMgr{
		getSamplingStrategyFunc: func(ctx context.Context, serviceName string) (*sampling.SamplingStrategyResponse, error) {
			return nil, errors.New("some error")
		},
	}

	rw := httptest.NewRecorder()
	req := &http.Request{
		URL: &url.URL{
			RawQuery: "service=foo",
		},
	}

	// test
	s.samplingStrategyHandler(rw, req)

	// verify
	body, _ := io.ReadAll(rw.Body)
	assert.Contains(t, string(body), "failed to get sampling strategy for service")
}
