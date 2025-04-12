// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/metadata"
)

func TestPushLogData(t *testing.T) {
	testCases := []struct {
		desc          string
		hints         map[string]any
		attrs         map[string]any
		res           map[string]any
		expectedLabel string
		expectedLine  string
	}{
		{
			desc: "with attribute to label and regular attribute",
			attrs: map[string]any{
				"host.name":   "guarana",
				"http.status": 200,
			},
			hints: map[string]any{
				"loki.attribute.labels": "host.name",
			},
			expectedLabel: `{exporter="OTLP", host_name="guarana"}`,
			expectedLine:  `{"traceid":"01020304000000000000000000000000","attributes":{"http.status":200}}`,
		},
		{
			desc: "with resource to label and regular resource",
			res: map[string]any{
				"host.name": "guarana",
				"region.az": "eu-west-1a",
			},
			hints: map[string]any{
				"loki.resource.labels": "host.name",
			},
			expectedLabel: `{exporter="OTLP", host_name="guarana"}`,
			expectedLine:  `{"traceid":"01020304000000000000000000000000","resources":{"region.az":"eu-west-1a"}}`,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			actualPushRequest := &push.PushRequest{}

			// prepare
			ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				encPayload, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				decPayload, err := snappy.Decode(nil, encPayload)
				assert.NoError(t, err)

				err = proto.Unmarshal(decPayload, actualPushRequest)
				assert.NoError(t, err)
			}))
			defer ts.Close()

			clientConfig := confighttp.NewDefaultClientConfig()
			clientConfig.Endpoint = ts.URL
			cfg := &Config{
				ClientConfig: clientConfig,
			}

			f := NewFactory()
			exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			ld := plog.NewLogs()
			ld.ResourceLogs().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
			ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).SetTraceID([16]byte{1, 2, 3, 4})

			// copy the attributes from the test case to the log entry
			if len(tC.attrs) > 0 {
				assert.NoError(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().FromRaw(tC.attrs))
			}
			if len(tC.res) > 0 {
				assert.NoError(t, ld.ResourceLogs().At(0).Resource().Attributes().FromRaw(tC.res))
			}

			// we can't use copy here, as the value (Value) will be used as string lookup later, so, we need to convert it to string now
			for k, v := range tC.hints {
				ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().PutStr(k, fmt.Sprintf("%v", v))
			}

			// test
			err = exp.ConsumeLogs(context.Background(), ld)
			require.NoError(t, err)

			// actualPushRequest is populated within the test http server, we check it here as assertions are better done at the
			// end of the test function
			assert.Len(t, actualPushRequest.Streams, 1)
			assert.Equal(t, tC.expectedLabel, actualPushRequest.Streams[0].Labels)

			assert.Len(t, actualPushRequest.Streams[0].Entries, 1)
			assert.Equal(t, tC.expectedLine, actualPushRequest.Streams[0].Entries[0].Line)

			// cleanup
			err = exp.Shutdown(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestLogsToLokiRequestWithGroupingByTenant(t *testing.T) {
	tests := []struct {
		desc     string
		logs     plog.Logs
		expected map[string]struct {
			line  string
			label string
		}
	}{
		{
			desc: "create a push request per tenant",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("loki.tenant", "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "1")
				logRecord.Attributes().PutInt("http.status", 200)

				sl = rl.ScopeLogs().AppendEmpty()
				logRecord = sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("loki.tenant", "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "2")
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]struct {
				line  string
				label string
			}{
				"1": {
					label: `{exporter="OTLP"}`,
					line:  `{"attributes":{"http.status":200,"tenant.id":"1"}}`,
				},
				"2": {
					label: `{exporter="OTLP"}`,
					line:  `{"attributes":{"http.status":200,"tenant.id":"2"}}`,
				},
			},
		},
		{
			desc: "tenant hint is not found in attributes",
			logs: func() plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("loki.tenant", "tenant.id")
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]struct {
				line  string
				label string
			}{
				"": {
					label: `{exporter="OTLP"}`,
					line:  `{"attributes":{"http.status":200}}`,
				},
			},
		},
		{
			desc: "use tenant resource attributes if both logs and resource attributes provided",
			logs: func() plog.Logs {
				logs := plog.NewLogs()

				rl := logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("loki.tenant", "tenant.id")
				rl.Resource().Attributes().PutStr("tenant.id", "1")

				sl := rl.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("loki.tenant", "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "11")
				logRecord.Attributes().PutInt("http.status", 200)

				rl = logs.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("loki.tenant", "tenant.id")
				rl.Resource().Attributes().PutStr("tenant.id", "2")

				sl = rl.ScopeLogs().AppendEmpty()
				logRecord = sl.LogRecords().AppendEmpty()
				logRecord.Attributes().PutStr("loki.tenant", "tenant.id")
				logRecord.Attributes().PutStr("tenant.id", "22")
				logRecord.Attributes().PutInt("http.status", 200)

				return logs
			}(),
			expected: map[string]struct {
				line  string
				label string
			}{
				"1": {
					label: `{exporter="OTLP"}`,
					line:  `{"attributes":{"http.status":200,"tenant.id":"11"},"resources":{"tenant.id":"1"}}`,
				},
				"2": {
					label: `{exporter="OTLP"}`,
					line:  `{"attributes":{"http.status":200,"tenant.id":"22"},"resources":{"tenant.id":"2"}}`,
				},
			},
		},
	}
	for _, tC := range tests {
		t.Run(tC.desc, func(t *testing.T) {
			actualPushRequestPerTenant := map[string]*push.PushRequest{}

			// prepare
			ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				encPayload, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				decPayload, err := snappy.Decode(nil, encPayload)
				assert.NoError(t, err)

				pr := &push.PushRequest{}
				err = proto.Unmarshal(decPayload, pr)
				assert.NoError(t, err)

				actualPushRequestPerTenant[r.Header.Get("X-Scope-OrgID")] = pr
			}))
			defer ts.Close()

			clientConfig := confighttp.NewDefaultClientConfig()
			clientConfig.Endpoint = ts.URL

			cfg := &Config{
				ClientConfig: clientConfig,
			}

			f := NewFactory()
			exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
			require.NoError(t, err)

			err = exp.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			// test
			err = exp.ConsumeLogs(context.Background(), tC.logs)
			require.NoError(t, err)

			// actualPushRequest is populated within the test http server, we check it here as assertions are better done at the
			// end of the test function
			assert.Len(t, actualPushRequestPerTenant, len(tC.expected))
			for tenant, request := range actualPushRequestPerTenant {
				pr, ok := tC.expected[tenant]
				assert.True(t, ok)

				expectedLabel := pr.label
				expectedLine := pr.line
				assert.Len(t, request.Streams, 1)
				assert.Equal(t, expectedLabel, request.Streams[0].Labels)

				assert.Len(t, request.Streams[0].Entries, 1)
				assert.Equal(t, expectedLine, request.Streams[0].Entries[0].Line)
			}
			// cleanup
			err = exp.Shutdown(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestExporter_encode(t *testing.T) {
	t.Run("with good proto", func(t *testing.T) {
		labels := model.LabelSet{
			model.LabelName("container_name"): model.LabelValue("mycontainer"),
		}
		entry := &push.Entry{
			Timestamp: time.Now(),
			Line:      "log message",
		}
		stream := push.Stream{
			Labels:  labels.String(),
			Entries: []push.Entry{*entry},
		}
		pr := &push.PushRequest{
			Streams: []push.Stream{stream},
		}

		req, err := encode(pr)
		require.NoError(t, err)
		_, err = snappy.Decode(nil, req)
		require.NoError(t, err)
	})

	t.Run("with bad proto", func(t *testing.T) {
		p := &badProtoForCoverage{
			Foo: "Bar",
		}

		req, err := encode(p)
		require.Error(t, err)
		require.Nil(t, req)
	})
}

type badProtoForCoverage struct {
	Foo string `protobuf:"bytes,1,opt,name=labels,proto3" json:"foo"`
}

func (p *badProtoForCoverage) Reset()         {}
func (p *badProtoForCoverage) String() string { return "" }
func (p *badProtoForCoverage) ProtoMessage()  {}
func (p *badProtoForCoverage) Marshal() (dAtA []byte, err error) {
	return nil, fmt.Errorf("this is a bad proto")
}
