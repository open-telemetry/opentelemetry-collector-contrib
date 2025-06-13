// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter/internal/metadata"
)

type senderTest struct {
	reqCounter *int32
	srv        *httptest.Server
	s          *sender
}

// prepareSenderTest prepares sender test environment.
// Provided cfgOpts additionally configure the sender after the sensible default
// for tests have been applied.
// The enclosed httptest.Server is closed automatically using test.Cleanup.
func prepareSenderTest(t *testing.T, compression configcompression.Type, cb []func(w http.ResponseWriter, req *http.Request), cfgOpts ...func(*Config)) *senderTest {
	var reqCounter int32
	// generate a test server so we can capture and inspect the request
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if len(cb) == 0 {
			return
		}

		if c := int(atomic.LoadInt32(&reqCounter)); assert.Greater(t, len(cb), c) {
			cb[c](w, req)
			atomic.AddInt32(&reqCounter, 1)
		}
	}))
	t.Cleanup(func() { testServer.Close() })

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = testServer.URL
	switch compression {
	case configcompression.TypeGzip:
		cfg.Compression = configcompression.TypeGzip
	case configcompression.TypeZstd:
		cfg.Compression = configcompression.TypeZstd
	case NoCompression:
		cfg.Compression = NoCompression
	case configcompression.TypeDeflate:
		cfg.Compression = configcompression.TypeDeflate
	default:
		cfg.Compression = configcompression.TypeGzip
	}
	cfg.Auth = nil
	httpSettings := cfg.ClientConfig
	host := componenttest.NewNopHost()
	client, err := httpSettings.ToClient(context.Background(), host, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	if err != nil {
		return nil
	}
	cfg.LogFormat = TextFormat
	cfg.MetricFormat = OTLPMetricFormat
	cfg.MaxRequestBodySize = 20_971_520
	for _, cfgOpt := range cfgOpts {
		cfgOpt(cfg)
	}

	require.NoError(t, err)

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	telemetryBuilder, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	return &senderTest{
		reqCounter: &reqCounter,
		srv:        testServer,
		s: newSender(
			logger,
			cfg,
			client,
			newPrometheusFormatter(),
			testServer.URL,
			testServer.URL,
			testServer.URL,
			func() string { return "" },
			func(string) {},
			component.ID{},
			telemetryBuilder,
		),
	}
}

func extractBody(t *testing.T, req *http.Request) string {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, req.Body)
	require.NoError(t, err)
	return buf.String()
}

func exampleLog() []plog.LogRecord {
	buffer := make([]plog.LogRecord, 1)
	buffer[0] = plog.NewLogRecord()
	buffer[0].Body().SetStr("Example log")

	return buffer
}

func exampleTwoLogs() []plog.LogRecord {
	buffer := make([]plog.LogRecord, 2)
	buffer[0] = plog.NewLogRecord()
	buffer[0].Body().SetStr("Example log")
	buffer[0].Attributes().PutStr("key1", "value1")
	buffer[0].Attributes().PutStr("key2", "value2")
	buffer[1] = plog.NewLogRecord()
	buffer[1].Body().SetStr("Another example log")
	buffer[1].Attributes().PutStr("key1", "value1")
	buffer[1].Attributes().PutStr("key2", "value2")

	return buffer
}

func exampleNLogs(n int) []plog.LogRecord {
	buffer := make([]plog.LogRecord, n)
	for i := 0; i < n; i++ {
		buffer[i] = plog.NewLogRecord()
		buffer[i].Body().SetStr("Example log")
	}

	return buffer
}

func decodeGzip(t *testing.T, data io.Reader) string {
	r, err := gzip.NewReader(data)
	require.NoError(t, err)

	var buf []byte
	buf, err = io.ReadAll(r)
	require.NoError(t, err)

	return string(buf)
}

func decodeZstd(t *testing.T, data io.Reader) string {
	r, err := zstd.NewReader(data)
	require.NoError(t, err)
	var buf []byte
	buf, err = io.ReadAll(r)
	require.NoError(t, err)

	return string(buf)
}

func decodeZlib(t *testing.T, data io.Reader) string {
	r, err := zlib.NewReader(data)
	if err != nil {
		return ""
	}
	var buf []byte
	buf, err = io.ReadAll(r)
	require.NoError(t, err)

	return string(buf)
}

func TestSendTrace(t *testing.T) {
	tracesMarshaler = ptrace.ProtoMarshaler{}
	td := exampleTrace()
	traceBody, err := tracesMarshaler.MarshalTraces(td)
	assert.NoError(t, err)
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, string(traceBody), body)
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-protobuf", req.Header.Get("Content-Type"))
		},
	})

	err = test.s.sendTraces(context.Background(), td)
	assert.NoError(t, err)
}

func TestSendLogs(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log\nAnother example log", body)
			assert.Equal(t, "key1=value, key2=value2", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs()
	logsRecords1 := slgs.AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")
	logsRecords2 := slgs.AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key1": "value", "key2": "value2"}),
	)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestSendLogsWithEmptyField(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log\nAnother example log", body)
			assert.Equal(t, "key1=value, key2=value2", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs()
	logsRecords1 := slgs.AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")
	logsRecords2 := slgs.AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key1": "value", "key2": "value2", "service": ""}),
	)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestSendLogsMultitype(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `{"lk1":"lv1","lk2":13}
["lv2",13]`
			assert.Equal(t, expected, body)
			assert.Equal(t, "key1=value, key2=value2", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs()
	logsRecords := slgs.AppendEmpty().LogRecords()
	attVal := pcommon.NewValueMap()
	attMap := attVal.Map()
	attMap.PutStr("lk1", "lv1")
	attMap.PutInt("lk2", 13)
	logRecord := logsRecords.AppendEmpty()
	attVal.CopyTo(logRecord.Body())

	attVal = pcommon.NewValueSlice()
	attArr := attVal.Slice()
	strVal := pcommon.NewValueStr("lv2")
	intVal := pcommon.NewValueInt(13)
	strVal.CopyTo(attArr.AppendEmpty())
	intVal.CopyTo(attArr.AppendEmpty())
	attVal.CopyTo(logsRecords.AppendEmpty().Body())

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key1": "value", "key2": "value2"}),
	)
	assert.NoError(t, err)

	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestSendLogsSplit(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
		},
	})
	test.s.config.MaxRequestBodySize = 10

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs()
	logsRecords1 := slgs.AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")
	logsRecords2 := slgs.AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fields{},
	)
	assert.NoError(t, err)

	assert.EqualValues(t, 2, *test.reqCounter)
}

func TestSendLogsSplitFailedOne(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := fmt.Fprintf(
				w,
				`{"id":"1TIRY-KGIVX-TPQRJ","errors":[{"code":"internal.error","message":"Internal server error."}]}`,
			)

			assert.NoError(t, err)

			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
		},
	})
	test.s.config.MaxRequestBodySize = 10
	test.s.config.LogFormat = TextFormat

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs()
	logsRecords1 := slgs.AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")
	logsRecords2 := slgs.AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	dropped, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fields{},
	)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error, id: 1TIRY-KGIVX-TPQRJ, errors: [{Code:internal.error Message:Internal server error.}]")
	assert.Len(t, dropped, 1)

	assert.EqualValues(t, 2, *test.reqCounter)
}

func TestSendLogsSplitFailedAll(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

			body := extractBody(t, req)
			assert.Equal(t, "Example log", body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			body := extractBody(t, req)
			assert.Equal(t, "Another example log", body)
		},
	})
	test.s.config.MaxRequestBodySize = 10
	test.s.config.LogFormat = TextFormat

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs()
	logsRecords1 := slgs.AppendEmpty().LogRecords()
	logsRecords1.AppendEmpty().Body().SetStr("Example log")
	logsRecords2 := slgs.AppendEmpty().LogRecords()
	logsRecords2.AppendEmpty().Body().SetStr("Another example log")

	dropped, err := test.s.sendNonOTLPLogs(context.Background(), rls, fields{})
	assert.EqualError(
		t,
		err,
		"failed sending data: status: 500 Internal Server Error\nfailed sending data: status: 404 Not Found",
	)
	assert.Len(t, dropped, 2)

	assert.EqualValues(t, 2, *test.reqCounter)
}

func TestSendLogsJsonConfig(t *testing.T) {
	twoLogsFunc := func() plog.ResourceLogs {
		rls := plog.NewResourceLogs()
		slgs := rls.ScopeLogs().AppendEmpty()
		log := slgs.LogRecords().AppendEmpty()

		log.Body().SetStr("Example log")
		log.Attributes().PutStr("key1", "value1")
		log.Attributes().PutStr("key2", "value2")

		log = slgs.LogRecords().AppendEmpty()
		log.Body().SetStr("Another example log")
		log.Attributes().PutStr("key1", "value1")
		log.Attributes().PutStr("key2", "value2")

		return rls
	}

	twoComplexBodyLogsFunc := func() plog.ResourceLogs {
		rls := plog.NewResourceLogs()
		slgs := rls.ScopeLogs().AppendEmpty()
		log := slgs.LogRecords().AppendEmpty()

		body := pcommon.NewValueMap().Map()
		body.PutStr("a", "b")
		body.PutBool("c", false)
		body.PutInt("d", 20)
		body.PutDouble("e", 20.5)

		f := pcommon.NewValueSlice()
		f.Slice().EnsureCapacity(4)
		f.Slice().AppendEmpty().SetStr("p")
		f.Slice().AppendEmpty().SetBool(true)
		f.Slice().AppendEmpty().SetInt(13)
		f.Slice().AppendEmpty().SetDouble(19.3)
		f.Slice().CopyTo(body.PutEmptySlice("f"))

		g := pcommon.NewValueMap()
		g.Map().PutStr("h", "i")
		g.Map().PutBool("j", false)
		g.Map().PutInt("k", 12)
		g.Map().PutDouble("l", 11.1)

		g.Map().CopyTo(body.PutEmptyMap("g"))

		log.Attributes().PutStr("m", "n")

		pcommon.NewValueMap().CopyTo(log.Body())
		body.CopyTo(log.Body().Map())

		return rls
	}

	testcases := []struct {
		name       string
		configOpts []func(*Config)
		bodyRegex  string
		logsFunc   func() plog.ResourceLogs
	}{
		{
			name: "default config",
			bodyRegex: `{"key1":"value1","key2":"value2","log":"Example log"}` +
				`\n` +
				`{"key1":"value1","key2":"value2","log":"Another example log"}`,
			logsFunc: twoLogsFunc,
		},
		{
			name:      "empty body",
			bodyRegex: `{"key1":"value1","key2":"value2"}`,

			logsFunc: func() plog.ResourceLogs {
				rls := plog.NewResourceLogs()
				slgs := rls.ScopeLogs().AppendEmpty()
				log := slgs.LogRecords().AppendEmpty()

				log.Attributes().PutStr("key1", "value1")
				log.Attributes().PutStr("key2", "value2")

				return rls
			},
		},
		{
			name: "complex body",
			bodyRegex: `{"log":{"a":"b","c":false,"d":20,"e":20.5,"f":\["p",true,13,19.3\],` +
				`"g":{"h":"i","j":false,"k":12,"l":11.1}},"m":"n"}`,
			logsFunc: twoComplexBodyLogsFunc,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
				func(_ http.ResponseWriter, req *http.Request) {
					body := extractBody(t, req)
					assert.Regexp(t, tc.bodyRegex, body)
				},
			}, tc.configOpts...)

			test.s.config.LogFormat = JSONFormat

			_, err := test.s.sendNonOTLPLogs(context.Background(),
				tc.logsFunc(),
				fields{},
			)
			assert.NoError(t, err)

			assert.EqualValues(t, 1, *test.reqCounter)
		})
	}
}

func TestSendLogsJson(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Example log"}`
			regex += `\n`
			regex += `{"key1":"value1","key2":"value2","log":"Another example log"}`
			assert.Regexp(t, regex, body)

			assert.Equal(t, "key=value", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	test.s.config.LogFormat = JSONFormat

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()
	log := slgs.LogRecords().AppendEmpty()

	log.Body().SetStr("Example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	log = slgs.LogRecords().AppendEmpty()
	log.Body().SetStr("Another example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key": "value"}),
	)
	assert.NoError(t, err)

	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestSendLogsJsonHTLM(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Example log"}`
			regex += `\n`
			regex += `{"key1":"value1","key2":"value2","log":"<p>Another example log</p>"}`
			assert.Regexp(t, regex, body)

			assert.Equal(t, "key=value", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	test.s.config.LogFormat = JSONFormat

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()
	log := slgs.LogRecords().AppendEmpty()

	log.Body().SetStr("Example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	log = slgs.LogRecords().AppendEmpty()
	log.Body().SetStr("<p>Another example log</p>")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key": "value"}),
	)
	assert.NoError(t, err)

	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestSendLogsJsonMultitype(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			var regex string
			regex += `{"key1":"value1","key2":"value2","log":{"lk1":"lv1","lk2":13}}`
			regex += `\n`
			regex += `{"key1":"value1","key2":"value2","log":\["lv2",13\]}`
			assert.Regexp(t, regex, body)

			assert.Equal(t, "key=value", req.Header.Get("X-Sumo-Fields"))
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
		},
	})
	test.s.config.LogFormat = JSONFormat

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()

	attVal := pcommon.NewValueMap()
	attMap := attVal.Map()
	attMap.PutStr("lk1", "lv1")
	attMap.PutInt("lk2", 13)

	log := slgs.LogRecords().AppendEmpty()
	attVal.CopyTo(log.Body())

	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	log = slgs.LogRecords().AppendEmpty()

	attVal = pcommon.NewValueSlice()
	attArr := attVal.Slice()
	strVal := pcommon.NewValueStr("lv2")
	intVal := pcommon.NewValueInt(13)

	strVal.CopyTo(attArr.AppendEmpty())
	intVal.CopyTo(attArr.AppendEmpty())

	attVal.CopyTo(log.Body())
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key": "value"}),
	)
	assert.NoError(t, err)

	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestSendLogsJsonSplit(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Example log"}`
			assert.Regexp(t, regex, body)
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Another example log"}`
			assert.Regexp(t, regex, body)
		},
	})
	test.s.config.LogFormat = JSONFormat
	test.s.config.MaxRequestBodySize = 10

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()
	log := slgs.LogRecords().AppendEmpty()

	log.Body().SetStr("Example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	log = slgs.LogRecords().AppendEmpty()
	log.Body().SetStr("Another example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	_, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key": "value"}),
	)
	assert.NoError(t, err)

	assert.EqualValues(t, 2, *test.reqCounter)
}

func TestSendLogsJsonSplitFailedOne(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

			body := extractBody(t, req)

			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Example log"}`
			assert.Regexp(t, regex, body)
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)

			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Another example log"}`
			assert.Regexp(t, regex, body)
		},
	})
	test.s.config.LogFormat = JSONFormat
	test.s.config.MaxRequestBodySize = 10

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()
	log := slgs.LogRecords().AppendEmpty()

	log.Body().SetStr("Example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	log = slgs.LogRecords().AppendEmpty()
	log.Body().SetStr("Another example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	dropped, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fieldsFromMap(map[string]string{"key": "value"}),
	)
	assert.EqualError(t, err, "failed sending data: status: 500 Internal Server Error")
	assert.Len(t, dropped, 1)

	assert.EqualValues(t, 2, *test.reqCounter)
}

func TestSendLogsJsonSplitFailedAll(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

			body := extractBody(t, req)

			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Example log"}`
			assert.Regexp(t, regex, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			body := extractBody(t, req)

			var regex string
			regex += `{"key1":"value1","key2":"value2","log":"Another example log"}`
			assert.Regexp(t, regex, body)
		},
	})
	test.s.config.LogFormat = JSONFormat
	test.s.config.MaxRequestBodySize = 10

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()
	log := slgs.LogRecords().AppendEmpty()

	log.Body().SetStr("Example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	log = slgs.LogRecords().AppendEmpty()
	log.Body().SetStr("Another example log")
	log.Attributes().PutStr("key1", "value1")
	log.Attributes().PutStr("key2", "value2")

	dropped, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fields{},
	)

	assert.EqualError(
		t,
		err,
		"failed sending data: status: 500 Internal Server Error\nfailed sending data: status: 404 Not Found",
	)
	assert.Len(t, dropped, 2)

	assert.EqualValues(t, 2, *test.reqCounter)
}

func TestSendLogsUnexpectedFormat(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, _ *http.Request) {
		},
	})
	test.s.config.LogFormat = "dummy"

	rls := plog.NewResourceLogs()
	slgs := rls.ScopeLogs().AppendEmpty()
	log := slgs.LogRecords().AppendEmpty()
	log.Body().SetStr("Example log")

	dropped, err := test.s.sendNonOTLPLogs(context.Background(),
		rls,
		fields{},
	)
	assert.Error(t, err)
	assert.Len(t, dropped, 1)
	assert.Equal(t, []plog.LogRecord{log}, dropped)
}

func TestSendLogsOTLP(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			//nolint:lll
			assert.Equal(t, "\n\x84\x01\n\x00\x12;\n\x00\x127*\r\n\vExample log2\x10\n\x04key1\x12\b\n\x06value12\x10\n\x04key2\x12\b\n\x06value2J\x00R\x00\x12C\n\x00\x12?*\x15\n\x13Another example log2\x10\n\x04key1\x12\b\n\x06value12\x10\n\x04key2\x12\b\n\x06value2J\x00R\x00", body)

			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/x-protobuf", req.Header.Get("Content-Type"))

			assert.Empty(t, req.Header.Get("X-Sumo-Fields"),
				"We should not get X-Sumo-Fields header when sending data with OTLP",
			)
			assert.Empty(t, req.Header.Get("X-Sumo-Category"),
				"We should not get X-Sumo-Category header when sending data with OTLP",
			)
			assert.Empty(t, req.Header.Get("X-Sumo-Name"),
				"We should not get X-Sumo-Name header when sending data with OTLP",
			)
			assert.Empty(t, req.Header.Get("X-Sumo-Host"),
				"We should not get X-Sumo-Host header when sending data with OTLP",
			)
		},
	})

	test.s.config.LogFormat = "otlp"

	l := plog.NewLogs()
	ls := l.ResourceLogs().AppendEmpty()

	logRecords := exampleTwoLogs()
	for i := 0; i < len(logRecords); i++ {
		logRecords[i].MoveTo(ls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty())
	}

	l.MarkReadOnly()

	assert.NoError(t, test.s.sendOTLPLogs(context.Background(), l))
	assert.EqualValues(t, 1, *test.reqCounter)
}

func TestLogsHandlesReceiverResponses(t *testing.T) {
	t.Run("json with too many fields logs a warning", func(t *testing.T) {
		test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
			func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, `{
					"status" : 200,
					"id" : "YBLR1-S2T29-MVXEJ",
					"code" : "bad.http.header.fields",
					"message" : "X-Sumo-Fields Warning: 14 key-value pairs are dropped as they are exceeding maximum key-value pair number limit 30."
				  }`)
			},
		}, func(c *Config) {
			c.LogFormat = JSONFormat
		})

		rls := plog.NewResourceLogs()
		rls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("Example log")

		var buffer bytes.Buffer
		writer := bufio.NewWriter(&buffer)
		test.s.logger = zap.New(
			zapcore.NewCore(
				zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
				zapcore.AddSync(writer),
				zapcore.DebugLevel,
			),
		)

		_, err := test.s.sendNonOTLPLogs(context.Background(),
			rls,
			fieldsFromMap(
				map[string]string{
					"cluster":         "abcaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"code":            "4222222222222222222222222222222222222222222222222222222222222222222222222222222222222",
					"component":       "apiserver",
					"endpoint":        "httpsaaaaaaaaaaaaaaaaaaa",
					"a":               "a",
					"b":               "b",
					"c":               "c",
					"d":               "d",
					"e":               "e",
					"f":               "f",
					"g":               "g",
					"q":               "q",
					"w":               "w",
					"r":               "r",
					"t":               "t",
					"y":               "y",
					"1":               "1",
					"2":               "2",
					"3":               "3",
					"4":               "4",
					"5":               "5",
					"6":               "6",
					"7":               "7",
					"8":               "8",
					"9":               "9",
					"10":              "10",
					"11":              "11",
					"12":              "12",
					"13":              "13",
					"14":              "14",
					"15":              "15",
					"16":              "16",
					"17":              "17",
					"18":              "18",
					"19":              "19",
					"20":              "20",
					"21":              "21",
					"22":              "22",
					"23":              "23",
					"24":              "24",
					"25":              "25",
					"26":              "26",
					"27":              "27",
					"28":              "28",
					"29":              "29",
					"_sourceName":     "test_source_name",
					"_sourceHost":     "test_source_host",
					"_sourceCategory": "test_source_category",
				}),
		)
		assert.NoError(t, writer.Flush())
		assert.NoError(t, err)
		assert.EqualValues(t, 1, *test.reqCounter)

		assert.Contains(t,
			buffer.String(),
			`There was an issue sending data	{`+
				`"status": "200 OK", `+
				`"id": "YBLR1-S2T29-MVXEJ", `+
				`"code": "bad.http.header.fields", `+
				`"message": "X-Sumo-Fields Warning: 14 key-value pairs are dropped as they are exceeding maximum key-value pair number limit 30."`,
		)
	})
}

func TestInvalidEndpoint(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){})

	test.s.dataURLLogs = ":"

	rls := plog.NewResourceLogs()
	rls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("Example log")

	_, err := test.s.sendNonOTLPLogs(context.Background(), rls, fields{})
	assert.EqualError(t, err, `parse ":": missing protocol scheme`)
}

func TestInvalidPostRequest(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){})

	test.s.dataURLLogs = ""
	rls := plog.NewResourceLogs()
	rls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("Example log")

	_, err := test.s.sendNonOTLPLogs(context.Background(), rls, fields{})
	assert.EqualError(t, err, `Post "": unsupported protocol scheme ""`)
}

func TestInvalidMetricFormat(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	test.s.config.MetricFormat = "invalid"

	err := test.s.send(context.Background(), MetricsPipeline, newCountingReader(0).withString(""), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, `unsupported metrics format: invalid`)
}

func TestInvalidPipeline(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){})
	defer func() { test.srv.Close() }()

	err := test.s.send(context.Background(), "invalidPipeline", newCountingReader(0).withString(""), newFields(pcommon.NewMap()))
	assert.EqualError(t, err, `unknown pipeline type: invalidPipeline`)
}

func TestSendCompressGzip(t *testing.T) {
	test := prepareSenderTest(t, configcompression.TypeGzip, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(http.StatusOK)
			if _, err := res.Write([]byte("")); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				assert.Fail(t, err.Error())
				return
			}
			body := decodeGzip(t, req.Body)
			assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))
			assert.Equal(t, "Some example log", body)
		},
	})

	reader := newCountingReader(0).withString("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, fields{})
	require.NoError(t, err)
}

func TestSendCompressGzipDeprecated(t *testing.T) {
	test := prepareSenderTest(t, "default", []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(http.StatusOK)
			if _, err := res.Write([]byte("")); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				assert.Fail(t, err.Error())
				return
			}
			body := decodeGzip(t, req.Body)
			assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))
			assert.Equal(t, "Some example log", body)
		},
	})

	reader := newCountingReader(0).withString("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, fields{})
	require.NoError(t, err)
}

func TestSendCompressZstd(t *testing.T) {
	test := prepareSenderTest(t, configcompression.TypeZstd, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(http.StatusOK)
			if _, err := res.Write([]byte("")); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				assert.Fail(t, err.Error())
				return
			}
			body := decodeZstd(t, req.Body)
			assert.Equal(t, "zstd", req.Header.Get("Content-Encoding"))
			assert.Equal(t, "Some example log", body)
		},
	})

	reader := newCountingReader(0).withString("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, fields{})
	require.NoError(t, err)
}

func TestSendCompressDeflate(t *testing.T) {
	test := prepareSenderTest(t, configcompression.TypeDeflate, []func(res http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, req *http.Request) {
			res.WriteHeader(http.StatusOK)
			if _, err := res.Write([]byte("")); err != nil {
				res.WriteHeader(http.StatusInternalServerError)
				assert.Fail(t, err.Error())
				return
			}
			body := decodeZlib(t, req.Body)
			assert.Equal(t, "deflate", req.Header.Get("Content-Encoding"))
			assert.Equal(t, "Some example log", body)
		},
	})

	reader := newCountingReader(0).withString("Some example log")

	err := test.s.send(context.Background(), LogsPipeline, reader, fields{})
	require.NoError(t, err)
}

func TestSendMetrics(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `` +
				`test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156920",url="http://example_url"} 124 1608124661166` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})

	test.s.config.MetricFormat = PrometheusFormat

	metricSum, attrs := exampleIntMetric()
	metricGauge, _ := exampleIntGaugeMetric()
	metrics := metricAndAttrsToPdataMetrics(
		attrs,
		metricSum, metricGauge,
	)
	metrics.MarkReadOnly()

	_, errs := test.s.sendNonOTLPMetrics(context.Background(), metrics)
	assert.Empty(t, errs)
}

func TestSendMetricsSplit(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `` +
				`test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156920",url="http://example_url"} 124 1608124661166` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
			assert.Equal(t, "otelcol", req.Header.Get("X-Sumo-Client"))
			assert.Equal(t, "application/vnd.sumologic.prometheus", req.Header.Get("Content-Type"))
		},
	})

	test.s.config.MetricFormat = PrometheusFormat

	metricSum, attrs := exampleIntMetric()
	metricGauge, _ := exampleIntGaugeMetric()
	metrics := metricAndAttrsToPdataMetrics(
		attrs,
		metricSum, metricGauge,
	)
	metrics.MarkReadOnly()

	_, errs := test.s.sendNonOTLPMetrics(context.Background(), metrics)
	assert.Empty(t, errs)
}

func TestSendOTLPHistogram(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			unmarshaler := pmetric.ProtoUnmarshaler{}
			body, err := io.ReadAll(req.Body)
			assert.NoError(t, err)
			metrics, err := unmarshaler.UnmarshalMetrics(body)
			assert.NoError(t, err)
			assert.Equal(t, 3, metrics.MetricCount())
			assert.Equal(t, 16, metrics.DataPointCount())
		},
	})

	defer func() { test.srv.Close() }()

	test.s.config.DecomposeOtlpHistograms = true

	metricHistogram, attrs := exampleHistogramMetric()

	metrics := pmetric.NewMetrics()

	rms := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rms.Resource().Attributes())
	metricHistogram.CopyTo(rms.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	metrics.MarkReadOnly()

	err := test.s.sendOTLPMetrics(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestSendMetricsSplitBySource(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `test.metric.data{test="test_value",test2="second_value",_sourceHost="value1"} 14500 1605534165000`
			assert.Equal(t, expected, body)
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `` +
				`gauge_metric_name{test="test_value",test2="second_value",_sourceHost="value2",remote_name="156920",url="http://example_url"} 124 1608124661166` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",_sourceHost="value2",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
		},
	})
	test.s.config.MetricFormat = PrometheusFormat

	metricSum, attrs := exampleIntMetric()
	metricGauge, _ := exampleIntGaugeMetric()

	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)

	rmsSum := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rmsSum.Resource().Attributes())
	rmsSum.Resource().Attributes().PutStr("_sourceHost", "value1")
	metricSum.CopyTo(rmsSum.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())

	rmsGauge := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rmsGauge.Resource().Attributes())
	rmsGauge.Resource().Attributes().PutStr("_sourceHost", "value2")
	metricGauge.CopyTo(rmsGauge.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	metrics.MarkReadOnly()

	_, errs := test.s.sendNonOTLPMetrics(context.Background(), metrics)
	assert.Empty(t, errs)
}

func TestSendMetricsSplitFailedOne(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

			body := extractBody(t, req)
			expected := `test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000`
			assert.Equal(t, expected, body)
		},
		func(_ http.ResponseWriter, req *http.Request) {
			body := extractBody(t, req)
			expected := `` +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156920",url="http://example_url"} 124 1608124661166` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
		},
	})
	test.s.config.MaxRequestBodySize = 10
	test.s.config.MetricFormat = PrometheusFormat

	metricSum, attrs := exampleIntMetric()
	metricGauge, _ := exampleIntGaugeMetric()
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)

	rmsSum := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rmsSum.Resource().Attributes())
	metricSum.CopyTo(rmsSum.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())

	rmsGauge := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rmsGauge.Resource().Attributes())
	metricGauge.CopyTo(rmsGauge.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	metrics.MarkReadOnly()

	dropped, errs := test.s.sendNonOTLPMetrics(context.Background(), metrics)
	assert.Len(t, errs, 1)
	assert.EqualError(t, errs[0], "failed sending data: status: 500 Internal Server Error")
	require.Equal(t, 1, dropped.MetricCount())
	assert.Equal(t, dropped.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0), metricSum)
}

func TestSendMetricsSplitFailedAll(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)

			body := extractBody(t, req)
			expected := `test.metric.data{test="test_value",test2="second_value"} 14500 1605534165000`
			assert.Equal(t, expected, body)
		},
		func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusNotFound)

			body := extractBody(t, req)
			expected := `` +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156920",url="http://example_url"} 124 1608124661166` + "\n" +
				`gauge_metric_name{test="test_value",test2="second_value",remote_name="156955",url="http://another_url"} 245 1608124662166`
			assert.Equal(t, expected, body)
		},
	})
	test.s.config.MaxRequestBodySize = 10
	test.s.config.MetricFormat = PrometheusFormat

	metricSum, attrs := exampleIntMetric()
	metricGauge, _ := exampleIntGaugeMetric()
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)

	rmsSum := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rmsSum.Resource().Attributes())
	metricSum.CopyTo(rmsSum.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())

	rmsGauge := metrics.ResourceMetrics().AppendEmpty()
	attrs.CopyTo(rmsGauge.Resource().Attributes())
	metricGauge.CopyTo(rmsGauge.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	metrics.MarkReadOnly()

	dropped, errs := test.s.sendNonOTLPMetrics(context.Background(), metrics)
	assert.Len(t, errs, 2)
	assert.EqualError(
		t,
		errs[0],
		"failed sending data: status: 500 Internal Server Error",
	)
	assert.EqualError(
		t,
		errs[1],
		"failed sending data: status: 404 Not Found",
	)
	require.Equal(t, 2, dropped.MetricCount())
	assert.Equal(t, dropped.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0), metricSum)
	assert.Equal(t, dropped.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0), metricGauge)
}

func TestSendMetricsUnexpectedFormat(t *testing.T) {
	// Expect no requests
	test := prepareSenderTest(t, NoCompression, nil)
	test.s.config.MetricFormat = "invalid"

	metricSum, attrs := exampleIntMetric()
	metrics := metricAndAttrsToPdataMetrics(attrs, metricSum)
	metrics.MarkReadOnly()

	dropped, errs := test.s.sendNonOTLPMetrics(context.Background(), metrics)
	assert.Len(t, errs, 1)
	assert.EqualError(t, errs[0], "unexpected metric format: invalid")
	require.Equal(t, 1, dropped.MetricCount())
	assert.Equal(t, dropped, metrics)
}

func TestBadRequestCausesPermanentError(t *testing.T) {
	test := prepareSenderTest(t, NoCompression, []func(w http.ResponseWriter, req *http.Request){
		func(res http.ResponseWriter, _ *http.Request) {
			res.WriteHeader(http.StatusBadRequest)
		},
	})
	test.s.config.MetricFormat = OTLPMetricFormat

	err := test.s.send(context.Background(), MetricsPipeline, newCountingReader(0).withString("malformed-request"), fields{})
	assert.True(t, consumererror.IsPermanent(err), "A '400 Bad Request' response from the server should result in a permanent error")
}
