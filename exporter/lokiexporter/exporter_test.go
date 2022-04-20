// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lokiexporter

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.6.1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/third_party/loki/logproto"
)

const (
	validEndpoint = "http://loki:3100/loki/api/v1/push"
)

var (
	testValidAttributesWithMapping = map[string]string{
		conventions.AttributeContainerName:  "container_name",
		conventions.AttributeK8SClusterName: "k8s_cluster_name",
		"severity":                          "severity",
	}
	testValidResourceWithMapping = map[string]string{
		"resource.name": "resource_name",
		"severity":      "severity",
	}
)

func createLogData(numberOfLogs int, attributes pcommon.Map) plog.Logs {
	logs := plog.NewLogs()
	sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.Body().SetStringVal("mylog")
		attributes.Range(func(k string, v pcommon.Value) bool {
			logRecord.Attributes().Insert(k, v)
			return true
		})
		logRecord.SetTimestamp(ts)
	}

	return logs
}

func TestExporter_new(t *testing.T) {
	t.Run("with valid config", func(t *testing.T) {
		config := &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: validEndpoint,
			},
			Labels: LabelsConfig{
				Attributes:         testValidAttributesWithMapping,
				ResourceAttributes: testValidResourceWithMapping,
			},
		}
		exp := newExporter(config, componenttest.NewNopTelemetrySettings())
		require.NotNil(t, exp)
	})
}

func TestExporter_pushLogData(t *testing.T) {

	genericReqTestFunc := func(t *testing.T, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		assert.Equal(t, "unit_tests", r.Header.Get("X-Scope-OrgID"))
		assert.Equal(t, "some_value", r.Header.Get("X-Custom-Header"))

		_, err = snappy.Decode(nil, body)
		if err != nil {
			t.Fatal(err)
		}
	}

	genericGenLogsFunc := func() plog.Logs {
		return createLogData(10,
			pcommon.NewMapFromRaw(map[string]interface{}{
				conventions.AttributeContainerName:  "api",
				conventions.AttributeK8SClusterName: "local",
				"resource.name":                     "myresource",
				"severity":                          "debug",
			}))
	}

	genericConfig := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
			Headers: map[string]string{
				"X-Custom-Header": "some_value",
			},
		},
		TenantID: "unit_tests",
		Labels: LabelsConfig{
			Attributes: map[string]string{
				conventions.AttributeContainerName:  "container_name",
				conventions.AttributeK8SClusterName: "k8s_cluster_name",
				"severity":                          "severity",
			},
			ResourceAttributes: map[string]string{
				"resource.name": "resource_name",
			},
		},
	}

	tests := []struct {
		name             string
		reqTestFunc      func(t *testing.T, r *http.Request)
		httpResponseCode int
		testServer       bool
		config           *Config
		genLogsFunc      func() plog.Logs
		errFunc          func(err error)
	}{
		{
			name:             "happy path",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: http.StatusOK,
			testServer:       true,
			genLogsFunc:      genericGenLogsFunc,
		},
		{
			name:             "server error",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: http.StatusInternalServerError,
			testServer:       true,
			genLogsFunc:      genericGenLogsFunc,
			errFunc: func(err error) {
				var e consumererror.Logs
				require.True(t, errors.As(err, &e))
				assert.Equal(t, 10, e.GetLogs().LogRecordCount())
			},
		},
		{
			name:             "server unavailable",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: 0,
			testServer:       false,
			genLogsFunc:      genericGenLogsFunc,
			errFunc: func(err error) {
				var e consumererror.Logs
				require.True(t, errors.As(err, &e))
				assert.Equal(t, 10, e.GetLogs().LogRecordCount())
			},
		},
		{
			name:             "with no matching attributes",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: http.StatusOK,
			testServer:       true,
			genLogsFunc: func() plog.Logs {
				return createLogData(10,
					pcommon.NewMapFromRaw(map[string]interface{}{
						"not.a.match": "random",
					}))
			},
			errFunc: func(err error) {
				require.True(t, consumererror.IsPermanent(err))
				require.Equal(t, "Permanent error: failed to transform logs into Loki log streams", err.Error())
			},
		},
		{
			name:             "with partial matching attributes",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: http.StatusOK,
			testServer:       true,
			genLogsFunc: func() plog.Logs {
				outLogs := plog.NewLogs()

				matchingLogs := createLogData(10,
					pcommon.NewMapFromRaw(map[string]interface{}{
						conventions.AttributeContainerName:  "api",
						conventions.AttributeK8SClusterName: "local",
						"severity":                          "debug",
					}))
				matchingLogs.ResourceLogs().MoveAndAppendTo(outLogs.ResourceLogs())

				nonMatchingLogs := createLogData(5,
					pcommon.NewMapFromRaw(map[string]interface{}{
						"not.a.match": "random",
					}))
				nonMatchingLogs.ResourceLogs().MoveAndAppendTo(outLogs.ResourceLogs())

				return outLogs
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.testServer {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tt.reqTestFunc != nil {
						tt.reqTestFunc(t, r)
					}
					w.WriteHeader(tt.httpResponseCode)
				}))
				defer server.Close()

				serverURL, err := url.Parse(server.URL)
				assert.NoError(t, err)
				tt.config.Endpoint = serverURL.String()
			}

			exp := newExporter(tt.config, componenttest.NewNopTelemetrySettings())
			require.NotNil(t, exp)
			err := exp.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			err = exp.pushLogData(context.Background(), tt.genLogsFunc())

			if tt.errFunc != nil {
				tt.errFunc(err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestExporter_logDataToLoki(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes: map[string]string{
				conventions.AttributeContainerName:  "container_name",
				conventions.AttributeK8SClusterName: "k8s_cluster_name",
				"severity":                          "severity",
			},
			ResourceAttributes: map[string]string{
				"resource.name": "resource_name",
			},
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	err := exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	t.Run("with attributes that match config", func(t *testing.T) {
		logs := plog.NewLogs()
		ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
		lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStringVal("log message")
		lr.Attributes().InsertString("not.in.config", "not allowed")
		lr.SetTimestamp(ts)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		expectedPr := &logproto.PushRequest{Streams: make([]logproto.Stream, 0)}
		require.Equal(t, 1, numDroppedLogs)
		require.Equal(t, expectedPr, pr)
	})

	t.Run("with partial attributes that match config", func(t *testing.T) {
		logs := plog.NewLogs()
		ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
		lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStringVal("log message")
		lr.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lr.Attributes().InsertString("severity", "info")
		lr.Attributes().InsertString("random.attribute", "random")
		lr.SetTimestamp(ts)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 1)
	})

	t.Run("with multiple logs and same attributes", func(t *testing.T) {
		logs := plog.NewLogs()
		ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
		sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		lr1 := sl.LogRecords().AppendEmpty()
		lr1.Body().SetStringVal("log message 1")
		lr1.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lr1.Attributes().InsertString(conventions.AttributeK8SClusterName, "mycluster")
		lr1.Attributes().InsertString("severity", "info")
		lr1.SetTimestamp(ts)

		lr2 := sl.LogRecords().AppendEmpty()
		lr2.Body().SetStringVal("log message 2")
		lr2.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lr2.Attributes().InsertString(conventions.AttributeK8SClusterName, "mycluster")
		lr2.Attributes().InsertString("severity", "info")
		lr2.SetTimestamp(ts)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 1)
		require.Len(t, pr.Streams[0].Entries, 2)
	})

	t.Run("with multiple logs and different attributes", func(t *testing.T) {
		logs := plog.NewLogs()
		ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
		sl := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

		lr1 := sl.LogRecords().AppendEmpty()
		lr1.Body().SetStringVal("log message 1")
		lr1.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer1")
		lr1.Attributes().InsertString(conventions.AttributeK8SClusterName, "mycluster1")
		lr1.Attributes().InsertString("severity", "debug")
		lr1.SetTimestamp(ts)

		lr2 := sl.LogRecords().AppendEmpty()
		lr2.Body().SetStringVal("log message 2")
		lr2.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer2")
		lr2.Attributes().InsertString(conventions.AttributeK8SClusterName, "mycluster2")
		lr2.Attributes().InsertString("severity", "error")
		lr2.SetTimestamp(ts)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 2)
		require.Len(t, pr.Streams[0].Entries, 1)
		require.Len(t, pr.Streams[1].Entries, 1)
	})

	t.Run("with attributes and resource attributes that match config", func(t *testing.T) {
		logs := plog.NewLogs()
		ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
		lr := logs.ResourceLogs().AppendEmpty()
		lr.Resource().Attributes().InsertString("not.in.config", "not allowed")

		lri := lr.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lri.Body().SetStringVal("log message")
		lri.Attributes().InsertString("not.in.config", "not allowed")
		lri.SetTimestamp(ts)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		expectedPr := &logproto.PushRequest{Streams: make([]logproto.Stream, 0)}
		require.Equal(t, 1, numDroppedLogs)
		require.Equal(t, expectedPr, pr)
	})

	t.Run("with attributes and resource attributes", func(t *testing.T) {
		logs := plog.NewLogs()
		ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
		lr := logs.ResourceLogs().AppendEmpty()
		lr.Resource().Attributes().InsertString("resource.name", "myresource")

		lri := lr.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lri.Body().SetStringVal("log message")
		lri.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lri.Attributes().InsertString("severity", "info")
		lri.Attributes().InsertString("random.attribute", "random")
		lri.SetTimestamp(ts)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 1)
	})

}

func TestExporter_convertAttributesToLabels(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes: map[string]string{
				conventions.AttributeContainerName:  "container_name",
				conventions.AttributeK8SClusterName: "k8s_cluster_name",
				"severity":                          "severity",
			},
			ResourceAttributes: map[string]string{
				"resource.name": "resource_name",
				"severity":      "severity",
			},
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	err := exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	t.Run("with attributes that match", func(t *testing.T) {
		am := pcommon.NewMap()
		am.InsertString(conventions.AttributeContainerName, "mycontainer")
		am.InsertString(conventions.AttributeK8SClusterName, "mycluster")
		am.InsertString("severity", "debug")
		ram := pcommon.NewMap()
		ram.InsertString("resource.name", "myresource")
		// this should overwrite log attribute of the same name
		ram.InsertString("severity", "info")

		ls, _ := exp.convertAttributesAndMerge(am, ram)
		expLs := model.LabelSet{
			model.LabelName("container_name"):   model.LabelValue("mycontainer"),
			model.LabelName("k8s_cluster_name"): model.LabelValue("mycluster"),
			model.LabelName("severity"):         model.LabelValue("info"),
			model.LabelName("resource_name"):    model.LabelValue("myresource"),
		}
		require.Equal(t, expLs, ls)
	})

	t.Run("with attribute matches and the value is a boolean", func(t *testing.T) {
		am := pcommon.NewMap()
		am.InsertBool("severity", false)
		ram := pcommon.NewMap()
		ls, _ := exp.convertAttributesAndMerge(am, ram)
		require.Nil(t, ls)
	})

	t.Run("with attribute that matches and the value is a double", func(t *testing.T) {
		am := pcommon.NewMap()
		am.InsertDouble("severity", float64(0))
		ram := pcommon.NewMap()
		ls, _ := exp.convertAttributesAndMerge(am, ram)
		require.Nil(t, ls)
	})

	t.Run("with attribute that matches and the value is an int", func(t *testing.T) {
		am := pcommon.NewMap()
		am.InsertInt("severity", 0)
		ram := pcommon.NewMap()
		ls, _ := exp.convertAttributesAndMerge(am, ram)
		require.Nil(t, ls)
	})

	t.Run("with attribute that matches and the value is null", func(t *testing.T) {
		am := pcommon.NewMap()
		am.InsertNull("severity")
		ram := pcommon.NewMap()
		ls, _ := exp.convertAttributesAndMerge(am, ram)
		require.Nil(t, ls)
	})
}

func TestExporter_convertLogBodyToEntry(t *testing.T) {
	res := pcommon.NewResource()
	res.Attributes().Insert("host.name", pcommon.NewValueString("something"))
	res.Attributes().Insert("pod.name", pcommon.NewValueString("something123"))

	lr := plog.NewLogRecord()
	lr.Body().SetStringVal("Payment succeeded")
	lr.SetTraceID(pcommon.NewTraceID([16]byte{1, 2, 3, 4}))
	lr.SetSpanID(pcommon.NewSpanID([8]byte{5, 6, 7, 8}))
	lr.SetSeverityText("DEBUG")
	lr.SetSeverityNumber(plog.SeverityNumberDEBUG)
	lr.Attributes().Insert("payment_method", pcommon.NewValueString("credit_card"))

	ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
	lr.SetTimestamp(ts)

	exp := newExporter(&Config{
		Labels: LabelsConfig{
			Attributes:         map[string]string{"payment_method": "payment_method"},
			ResourceAttributes: map[string]string{"pod.name": "pod.name"},
		},
	}, componenttest.NewNopTelemetrySettings())
	entry, _ := exp.convertLogBodyToEntry(lr, res)

	expEntry := &logproto.Entry{
		Timestamp: time.Unix(0, int64(lr.Timestamp())),
		Line:      "severity=DEBUG severityN=5 traceID=01020304000000000000000000000000 spanID=0506070800000000 host.name=something Payment succeeded",
	}
	require.NotNil(t, entry)
	require.Equal(t, expEntry, entry)
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

func TestExporter_encode(t *testing.T) {
	t.Run("with good proto", func(t *testing.T) {
		labels := model.LabelSet{
			model.LabelName("container_name"): model.LabelValue("mycontainer"),
		}
		entry := &logproto.Entry{
			Timestamp: time.Now(),
			Line:      "log message",
		}
		stream := logproto.Stream{
			Labels:  labels.String(),
			Entries: []logproto.Entry{*entry},
		}
		pr := &logproto.PushRequest{
			Streams: []logproto.Stream{stream},
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

func TestExporter_startReturnsNillWhenValidConfig(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes:         testValidAttributesWithMapping,
			ResourceAttributes: testValidResourceWithMapping,
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.NoError(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_startReturnsErrorWhenInvalidHttpClientSettings(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
			CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
				return nil, fmt.Errorf("this causes HTTPClientSettings.ToClient() to error")
			},
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.Error(t, exp.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_stopAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes:         testValidAttributesWithMapping,
			ResourceAttributes: testValidResourceWithMapping,
		},
	}
	exp := newExporter(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, exp)
	require.NoError(t, exp.stop(context.Background()))
}

func TestExporter_convertLogtoJSONEntry(t *testing.T) {
	ts := pcommon.Timestamp(int64(1) * time.Millisecond.Nanoseconds())
	lr := plog.NewLogRecord()
	lr.Body().SetStringVal("log message")
	lr.SetTimestamp(ts)
	res := pcommon.NewResource()
	res.Attributes().Insert("host.name", pcommon.NewValueString("something"))

	exp := newExporter(&Config{}, componenttest.NewNopTelemetrySettings())
	entry, err := exp.convertLogToJSONEntry(lr, res)
	expEntry := &logproto.Entry{
		Timestamp: time.Unix(0, int64(lr.Timestamp())),
		Line:      `{"body":"log message","resources":{"host.name":"something"}}`,
	}
	require.Nil(t, err)
	require.NotNil(t, entry)
	require.Equal(t, expEntry, entry)
}

func TestConvertRecordAttributesToLabels(t *testing.T) {
	testCases := []struct {
		desc     string
		lr       plog.LogRecord
		expected model.LabelSet
	}{
		{
			desc: "traceID",
			lr: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetTraceID(pcommon.NewTraceID([16]byte{1, 2, 3, 4}))
				return lr
			}(),
			expected: func() model.LabelSet {
				ls := model.LabelSet{}
				ls[model.LabelName("traceID")] = model.LabelValue("01020304000000000000000000000000")
				return ls
			}(),
		},
		{
			desc: "spanID",
			lr: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))
				return lr
			}(),
			expected: func() model.LabelSet {
				ls := model.LabelSet{}
				ls[model.LabelName("spanID")] = model.LabelValue("0102030400000000")
				return ls
			}(),
		},
		{
			desc: "severity",
			lr: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetSeverityText("DEBUG")
				return lr
			}(),
			expected: func() model.LabelSet {
				ls := model.LabelSet{}
				ls[model.LabelName("severity")] = model.LabelValue("DEBUG")
				return ls
			}(),
		},
		{
			desc: "severityN",
			lr: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.SetSeverityNumber(plog.SeverityNumberDEBUG)
				return lr
			}(),
			expected: func() model.LabelSet {
				ls := model.LabelSet{}
				ls[model.LabelName("severityN")] = model.LabelValue(plog.SeverityNumberDEBUG.String())
				return ls
			}(),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			exp := newExporter(&Config{
				Labels: LabelsConfig{
					RecordAttributes: map[string]string{
						tC.desc: tC.desc,
					},
				},
			}, componenttest.NewNopTelemetrySettings())

			ls := exp.convertRecordAttributesToLabels(tC.lr)

			assert.Equal(t, tC.expected, ls)
		})
	}
}

func TestExporter_timestampFromLogRecord(t *testing.T) {
	timeNow := time.Now()
	tests := []struct {
		name              string
		timestamp         time.Time
		observedTimestamp time.Time
		expectedTimestamp time.Time
	}{
		{
			name:              "timestamp is correct",
			timestamp:         timeNow,
			expectedTimestamp: timeNow,
		},
		{
			name:              "timestamp is empty",
			observedTimestamp: timeNow,
			expectedTimestamp: timeNow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			if !tt.timestamp.IsZero() {
				lr.SetTimestamp(pcommon.NewTimestampFromTime(tt.timestamp))
			}
			if !tt.observedTimestamp.IsZero() {
				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(tt.observedTimestamp))
			}

			assert.Equal(t, time.Unix(0, int64(pcommon.NewTimestampFromTime(tt.expectedTimestamp))), timestampFromLogRecord(lr))
		})
	}
}
