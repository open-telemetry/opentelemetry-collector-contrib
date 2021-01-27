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

package newrelic

import (
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/buffer"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
	"github.com/opentelemetry/opentelemetry-log-collection/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewRelicConfigBuild(t *testing.T) {
	t.Run("OutputConfigError", func(t *testing.T) {
		cfg := NewNewRelicOutputConfig("test")
		cfg.OperatorType = ""
		_, err := cfg.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing required `type` field")
	})

	t.Run("MissingKey", func(t *testing.T) {
		cfg := NewNewRelicOutputConfig("test")
		_, err := cfg.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Equal(t, err.Error(), "one of 'api_key' or 'license_key' is required")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		cfg := NewNewRelicOutputConfig("test")
		cfg.LicenseKey = "testkey"
		cfg.BaseURI = `%^&*($@)`
		_, err := cfg.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not a valid URL")
	})
}

func TestNewRelicOutput(t *testing.T) {
	cases := []struct {
		name     string
		cfgMod   func(*NewRelicOutputConfig)
		input    []*entry.Entry
		expected string
	}{
		{
			"Simple",
			nil,
			[]*entry.Entry{{
				Timestamp: time.Date(2016, 10, 10, 8, 58, 52, 0, time.UTC),
				Record:    "test",
			}},
			`[{"common":{"attributes":{"plugin":{"type":"stanza","version":"unknown"}}},"logs":[{"timestamp":1476089932000,"attributes":{"labels":null,"record":"test","resource":null,"severity":"default"},"message":"test"}]}]` + "\n",
		},
		{
			"Multi",
			nil,
			[]*entry.Entry{{
				Timestamp: time.Date(2016, 10, 10, 8, 58, 52, 0, time.UTC),
				Record:    "test1",
			}, {
				Timestamp: time.Date(2016, 10, 10, 8, 58, 52, 0, time.UTC),
				Record:    "test2",
			}},
			`[{"common":{"attributes":{"plugin":{"type":"stanza","version":"unknown"}}},"logs":[{"timestamp":1476089932000,"attributes":{"labels":null,"record":"test1","resource":null,"severity":"default"},"message":"test1"},{"timestamp":1476089932000,"attributes":{"labels":null,"record":"test2","resource":null,"severity":"default"},"message":"test2"}]}]` + "\n",
		},
		{
			"CustomMessage",
			func(cfg *NewRelicOutputConfig) {
				cfg.MessageField = entry.NewRecordField("log")
			},
			[]*entry.Entry{{
				Timestamp: time.Date(2016, 10, 10, 8, 58, 52, 0, time.UTC),
				Record: map[string]interface{}{
					"log":     "testlog",
					"message": "testmessage",
				},
			}},
			`[{"common":{"attributes":{"plugin":{"type":"stanza","version":"unknown"}}},"logs":[{"timestamp":1476089932000,"attributes":{"labels":null,"record":{"log":"testlog","message":"testmessage"},"resource":null,"severity":"default"},"message":"testlog"}]}]` + "\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ln := newListener()
			addr, err := ln.start()
			require.NoError(t, err)
			defer ln.stop()

			cfg := NewNewRelicOutputConfig("test")
			cfg.BufferConfig = buffer.Config{
				Builder: func() buffer.Builder {
					cfg := buffer.NewMemoryBufferConfig()
					cfg.MaxChunkDelay = helper.NewDuration(50 * time.Millisecond)
					return cfg
				}(),
			}
			cfg.BaseURI = fmt.Sprintf("http://%s/log/v1", addr)
			cfg.APIKey = "testkey"
			if tc.cfgMod != nil {
				tc.cfgMod(cfg)
			}

			ops, err := cfg.Build(testutil.NewBuildContext(t))
			require.NoError(t, err)
			op := ops[0]
			require.NoError(t, op.Start())
			for _, entry := range tc.input {
				require.NoError(t, op.Process(context.Background(), entry))
			}
			defer op.Stop()

			expectTestConnection(t, ln)
			expectRequestBody(t, ln, tc.expected)
		})
	}

	t.Run("FailedTestConnection", func(t *testing.T) {
		cfg := NewNewRelicOutputConfig("test")
		cfg.BaseURI = "http://localhost/log/v1"
		cfg.APIKey = "testkey"

		ops, err := cfg.Build(testutil.NewBuildContext(t))
		require.NoError(t, err)
		op := ops[0]
		err = op.Start()
		require.Error(t, err)
	})
}

func expectTestConnection(t *testing.T, ln *listener) {
	testConnection := `[{"common":{"attributes":{"plugin":{"type":"stanza","version":"unknown"}}},"logs":[]}]` + "\n"
	expectRequestBody(t, ln, testConnection)
}

func expectRequestBody(t *testing.T, ln *listener, expected string) {
	select {
	case body := <-ln.requestBodies:
		require.Equal(t, expected, string(body))
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for test connection")
	}
}

type listener struct {
	server        *http.Server
	requestBodies chan []byte
}

func newListener() *listener {
	requests := make(chan []byte, 100)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handle(requests))

	return &listener{
		server: &http.Server{
			Handler: mux,
		},
		requestBodies: requests,
	}
}

func (l *listener) start() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go func() {
		l.server.Serve(ln)
	}()

	return ln.Addr().String(), nil
}

func (l *listener) stop() {
	l.server.Shutdown(context.Background())
}

func handle(ch chan []byte) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
		rw.Write([]byte(`{}`))

		rd, err := gzip.NewReader(req.Body)
		if err != nil {
			panic(err)
		}
		body, err := ioutil.ReadAll(rd)
		if err != nil {
			panic(err)
		}
		req.Body.Close()

		ch <- body
	}
}
