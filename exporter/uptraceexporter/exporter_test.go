// Copyright OpenTelemetry Authors
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

package uptraceexporter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/uptrace-go/spanexp"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/otel/label"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/uptraceexporter/testdata"
)

func TestNewTraceExporterEmptyConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.Error(t, err)
	require.Nil(t, exp)
}

func TestNewTraceExporterEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "_"
	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.Error(t, err)
	require.Nil(t, exp)
}

func TestTraceExporterEmptyTraces(t *testing.T) {
	ctx := context.Background()

	cfg := createDefaultConfig().(*Config)
	cfg.DSN = "https://key@api.uptrace.dev/1"

	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, exp)

	dropped, err := exp.pushTraceData(ctx, pdata.NewTraces())
	require.NoError(t, err)
	require.Zero(t, dropped)

	err = exp.Shutdown(ctx)
	require.NoError(t, err)
}

func TestTraceExporterGenTraces(t *testing.T) {
	type In struct {
		Spans []spanexp.Span `msgpack:"spans"`
	}

	var in In

	handler := func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, "application/msgpack", req.Header.Get("Content-Type"))
		require.Equal(t, "zstd", req.Header.Get("Content-Encoding"))

		zr, err := zstd.NewReader(req.Body)
		require.NoError(t, err)

		dec := msgpack.NewDecoder(zr)
		err = dec.Decode(&in)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}

	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(handler))
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.DSN = fmt.Sprintf("%s://key@%s/1", u.Scheme, u.Host)

	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, exp)

	dropped, err := exp.pushTraceData(
		ctx, testdata.GenerateTraceDataTwoSpansSameResource())
	require.NoError(t, err)
	require.Zero(t, dropped)

	err = exp.Shutdown(ctx)
	require.NoError(t, err)

	var traceID [16]byte
	traceID[0] = 0xff

	require.Equal(t, In{
		Spans: []spanexp.Span{
			{
				ID:        506097522914230528,
				ParentID:  18446744073709551615,
				TraceID:   traceID,
				Name:      "operationA",
				Kind:      "internal",
				StartTime: 1581452772000000321,
				EndTime:   1581452773000000789,

				Resource: []label.KeyValue{
					label.String("resource-attr", "resource-attr-val-1"),
				},

				StatusCode:    "error",
				StatusMessage: "status-cancelled",

				Events: []spanexp.Event{
					{
						Name: "event-with-attr",
						Attrs: []label.KeyValue{
							label.String("span-event-attr", "span-event-attr-val"),
						},
						Time: 1581452773000000123,
					},
					{
						Name: "event",
						Time: 1581452773000000123,
					},
				},
			},
			{
				Name:      "operationB",
				Kind:      "server",
				StartTime: 1581452772000000321,
				EndTime:   1581452773000000789,

				Resource: []label.KeyValue{
					label.String("resource-attr", "resource-attr-val-1"),
				},

				StatusCode:    "ok",
				StatusMessage: "",

				Links: []spanexp.Link{
					{
						Attrs: []label.KeyValue{
							label.String("span-link-attr", "span-link-attr-val"),
						},
					},
					{},
				},
			},
		},
	}, in)
}
