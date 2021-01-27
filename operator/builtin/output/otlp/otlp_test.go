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

package otlp

import (
	"context"
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

func TestOTLPConfigBuild(t *testing.T) {
	t.Run("OutputConfigError", func(t *testing.T) {
		cfg := NewOTLPOutputConfig("test")
		cfg.OperatorType = ""
		_, err := cfg.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing required `type` field")
	})

	t.Run("MissingEndpoint", func(t *testing.T) {
		cfg := NewOTLPOutputConfig("test")
		cfg.Endpoint = ""
		_, err := cfg.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Equal(t, err.Error(), "'endpoint' is required")
	})

	t.Run("InvalidEndpoint", func(t *testing.T) {
		cfg := NewOTLPOutputConfig("test")
		cfg.Endpoint = `%^&*($@)`
		_, err := cfg.Build(testutil.NewBuildContext(t))
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not a valid URL")
	})
}

func TestOTLPOutput(t *testing.T) {
	cases := []struct {
		name   string
		cfgMod func(*OTLPOutputConfig)
		input  []*entry.Entry
	}{
		{
			"Simple",
			nil,
			[]*entry.Entry{{
				Timestamp: time.Date(2016, 10, 10, 8, 58, 52, 0, time.UTC),
				Record:    "test",
			}},
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
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ln := newListener()
			addr, err := ln.start()
			require.NoError(t, err)
			defer ln.stop()

			cfg := NewOTLPOutputConfig("test")
			cfg.BufferConfig.Builder.(*buffer.MemoryBufferConfig).MaxChunkDelay = helper.NewDuration(50 * time.Millisecond)
			cfg.Endpoint = addr
			cfg.TLSSetting.Insecure = true
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

			// TODO test connection?
			// expectTestConnection(t, ln)
			expected, err := Convert(tc.input).ToOtlpProtoBytes()
			require.NoError(t, err)
			expectRequestBody(t, ln, expected)
		})
	}
}

func expectRequestBody(t *testing.T, ln *listener, expected []byte) {
	select {
	case body := <-ln.requestBodies:
		require.Equal(t, expected, body)
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

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)
		}
		req.Body.Close()

		ch <- body
	}
}
