// Copyright 2020, OpenTelemetry Authors
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

package nginxreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/codes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

func TestScraper(t *testing.T) {
	nginxMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/status" {
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(`Active connections: 291
server accepts handled requests
 16630948 16630948 31070465
Reading: 6 Writing: 179 Waiting: 106
`))
			return
		}
		rw.WriteHeader(404)
	}))
	sc := newNginxScraper(zap.NewNop(), &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: nginxMock.URL + "/status",
		},
	})
	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	rms, err := sc.scrape(context.Background())
	require.Nil(t, err)

	require.Equal(t, 1, rms.Len())
	rm := rms.At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())

	ilm := ilms.At(0)
	ms := ilm.Metrics()

	require.Equal(t, 4, ms.Len())

	metricValues := make(map[string]int64, 7)

	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)

		switch m.DataType() {
		case pdata.MetricDataTypeGauge:
			dps := m.Gauge().DataPoints()
			require.Equal(t, 4, dps.Len())
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				state, _ := dp.LabelsMap().Get(metadata.L.State)
				label := fmt.Sprintf("%s state:%s", m.Name(), state)
				metricValues[label] = dp.IntVal()
			}
		case pdata.MetricDataTypeSum:
			dps := m.Sum().DataPoints()
			require.Equal(t, 1, dps.Len())
			metricValues[m.Name()] = dps.At(0).IntVal()
		}
	}

	require.Equal(t, map[string]int64{
		"nginx.connections_accepted":              16630948,
		"nginx.connections_handled":               16630948,
		"nginx.requests":                          31070465,
		"nginx.connections_current state:active":  291,
		"nginx.connections_current state:reading": 6,
		"nginx.connections_current state:writing": 179,
		"nginx.connections_current state:waiting": 106,
	}, metricValues)
}

func TestScraperError(t *testing.T) {
	nginxMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/status" {
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(`Bad status page`))
			return
		}
		rw.WriteHeader(404)
	}))
	t.Run("404", func(t *testing.T) {
		sc := newNginxScraper(zap.NewNop(), &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: nginxMock.URL + "/badpath",
			},
		})
		err := sc.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		_, err = sc.scrape(context.Background())
		require.Equal(t, errors.New("expected 200 response, got 404"), err)
	})

	t.Run("parse error", func(t *testing.T) {
		sc := newNginxScraper(zap.NewNop(), &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: nginxMock.URL + "/status",
			},
		})
		err := sc.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		_, err = sc.scrape(context.Background())
		require.Equal(t, errors.New("failed to parse response body \"Bad status page\": invalid input \"Bad status page\""), err)
	})
}

func TestScraperLogs(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	nginxMock := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/bad_get" {
			time.Sleep(11 * time.Millisecond)
			return
		}
		if req.URL.Path == "/bad_response_body" {
			rw.Header().Set("Content-Length", "1")
			return
		}
		if req.URL.Path == "/bad_parse" {
			_, _ = rw.Write([]byte(`Bad status page`))
			return
		}
		if req.URL.Path == "/400" {
			rw.WriteHeader(400)
			return
		}
		if req.URL.Path == "/401" {
			rw.WriteHeader(401)
			return
		}
		if req.URL.Path == "/403" {
			rw.WriteHeader(403)
			return
		}
		if req.URL.Path == "/404" {
			rw.WriteHeader(404)
			return
		}
		if req.URL.Path == "/429" {
			rw.WriteHeader(429)
			return
		}
		if req.URL.Path == "/502" {
			rw.WriteHeader(502)
			return
		}
		if req.URL.Path == "/503" {
			rw.WriteHeader(503)
			return
		}
		if req.URL.Path == "/504" {
			rw.WriteHeader(504)
			return
		}
		defaultPort, _ := strconv.Atoi(req.URL.Path[1:])
		rw.WriteHeader(defaultPort)
	}))
	http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = 10 * time.Millisecond
	nginxMock.Listener.Close()
	nginxMock.Listener = l
	nginxMock.Start()

	testCases := []struct {
		desc     string
		endpoint string
		expected []observer.LoggedEntry
	}{
		{
			desc:     "failed to get logging",
			endpoint: "/bad_get",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("failed to get http://%s/bad_get: Get \"http://%s/bad_get\": net/http: timeout awaiting response headers", l.Addr().String(), l.Addr().String())),
						zap.String("status_code", codes.Unavailable.String()),
					},
				},
			},
		},
		{
			desc:     "400 logging",
			endpoint: "/400",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 400")),
						zap.String("status_code", codes.Internal.String()),
					},
				},
			},
		},
		{
			desc:     "401 logging",
			endpoint: "/401",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 401")),
						zap.String("status_code", codes.Unauthenticated.String()),
					},
				},
			},
		},
		{
			desc:     "403 logging",
			endpoint: "/403",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 403")),
						zap.String("status_code", codes.PermissionDenied.String()),
					},
				},
			},
		},
		{
			desc:     "404 logging",
			endpoint: "/404",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 404")),
						zap.String("status_code", codes.Unimplemented.String()),
					},
				},
			},
		},
		{
			desc:     "429 logging",
			endpoint: "/429",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 429")),
						zap.String("status_code", codes.Unavailable.String()),
					},
				},
			},
		},
		{
			desc:     "502 logging",
			endpoint: "/502",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 502")),
						zap.String("status_code", codes.Unavailable.String()),
					},
				},
			},
		},
		{
			desc:     "503 logging",
			endpoint: "/503",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 503")),
						zap.String("status_code", codes.Unavailable.String()),
					},
				},
			},
		},
		{
			desc:     "504 logging",
			endpoint: "/504",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 504")),
						zap.String("status_code", codes.Unavailable.String()),
					},
				},
			},
		},
		{
			desc:     "505 default logging",
			endpoint: "/505",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("expected 200 response, got 505")),
						zap.String("status_code", codes.Unknown.String()),
					},
				},
			},
		},
		{
			desc:     "bad response body",
			endpoint: "/bad_response_body",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("failed to read the response body: unexpected EOF")),
						zap.String("status_code", codes.Internal.String()),
					},
				},
			},
		},
		{
			desc:     "bad status page",
			endpoint: "/bad_parse",
			expected: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.ErrorLevel,
						Message: "nginx"},
					Context: []zapcore.Field{
						zap.Error(fmt.Errorf("failed to parse response body \"Bad status page\": invalid input \"Bad status page\"")),
						zap.String("status_code", codes.Internal.String()),
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			obs, logs := observer.New(zap.ErrorLevel)

			sc := newNginxScraper(zap.New(obs), &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: nginxMock.URL + tC.endpoint,
				},
			})

			err := sc.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			_, err = sc.scrape(context.Background())
			require.Error(t, err)

			require.Equal(t, 1, logs.Len())
			require.Equal(t, tC.expected, logs.AllUntimed())
		})
	}
	nginxMock.Close()
}

func TestScraperFailedStart(t *testing.T) {
	obs, logs := observer.New(zap.ErrorLevel)
	sc := newNginxScraper(zap.New(obs), &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "localhost:8080",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "/non/existent",
				},
			},
		},
	})
	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)

	require.Equal(t, 1, logs.Len())
	errorsMap := logs.AllUntimed()[0].ContextMap()
	require.Equal(t, "failed to load TLS config: failed to load CA CertPool: failed to load CA /non/existent: open /non/existent: no such file or directory", errorsMap["error"])
	require.Equal(t, codes.InvalidArgument.String(), errorsMap["status_code"])
}
