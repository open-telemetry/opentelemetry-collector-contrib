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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/third_party/loki/logproto"
)

const (
	validEndpoint = "http://loki:3100/loki/api/v1/push"
)

var (
	testValidAttributesWithMapping = map[string]string{
		conventions.AttributeContainerName: "container_name",
		conventions.AttributeK8sCluster:    "k8s_cluster_name",
		"severity":                         "severity",
	}
)

func createLogData(numberOfLogs int, attributes pdata.AttributeMap) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	rl := logs.ResourceLogs().At(0)
	rl.InstrumentationLibraryLogs().Resize(1)
	ill := rl.InstrumentationLibraryLogs().At(0)

	for i := 0; i < numberOfLogs; i++ {
		ts := pdata.TimestampUnixNano(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := pdata.NewLogRecord()
		logRecord.Body().SetStringVal("mylog")
		attributes.ForEach(func(k string, v pdata.AttributeValue) {
			logRecord.Attributes().Insert(k, v)
		})
		logRecord.SetTimestamp(ts)

		ill.Logs().Append(logRecord)
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
				Attributes: testValidAttributesWithMapping,
			},
		}
		exp, err := newExporter(config, zap.NewNop())
		require.NoError(t, err)
		require.NotNil(t, exp)
	})

	t.Run("with invalid HTTPClientSettings", func(t *testing.T) {
		config := &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "",
				CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
					return nil, fmt.Errorf("this causes HTTPClientSettings.ToClient() to error")
				},
			},
		}
		_, err := newExporter(config, zap.NewNop())
		require.Error(t, err)
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

	genericGenLogsFunc := func() pdata.Logs {
		return createLogData(10,
			pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
				conventions.AttributeContainerName: pdata.NewAttributeValueString("api"),
				conventions.AttributeK8sCluster:    pdata.NewAttributeValueString("local"),
				"severity":                         pdata.NewAttributeValueString("debug"),
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
				conventions.AttributeContainerName: "container_name",
				conventions.AttributeK8sCluster:    "k8s_cluster_name",
				"severity":                         "severity",
			},
		},
	}

	tests := []struct {
		name             string
		reqTestFunc      func(t *testing.T, r *http.Request)
		httpResponseCode int
		testServer       bool
		config           *Config
		genLogsFunc      func() pdata.Logs
		numDroppedLogs   int
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
			numDroppedLogs:   10,
			errFunc: func(err error) {
				e := err.(consumererror.PartialError)
				require.Equal(t, 10, e.GetLogs().LogRecordCount())
			},
		},
		{
			name:             "server unavailable",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: 0,
			testServer:       false,
			genLogsFunc:      genericGenLogsFunc,
			numDroppedLogs:   10,
			errFunc: func(err error) {
				e := err.(consumererror.PartialError)
				require.Equal(t, 10, e.GetLogs().LogRecordCount())
			},
		},
		{
			name:             "with no matching attributes",
			reqTestFunc:      genericReqTestFunc,
			config:           genericConfig,
			httpResponseCode: http.StatusOK,
			testServer:       true,
			genLogsFunc: func() pdata.Logs {
				return createLogData(10,
					pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
						"not.a.match": pdata.NewAttributeValueString("random"),
					}))
			},
			numDroppedLogs: 10,
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
			genLogsFunc: func() pdata.Logs {
				outLogs := pdata.NewLogs()

				matchingLogs := createLogData(10,
					pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
						conventions.AttributeContainerName: pdata.NewAttributeValueString("api"),
						conventions.AttributeK8sCluster:    pdata.NewAttributeValueString("local"),
						"severity":                         pdata.NewAttributeValueString("debug"),
					}))
				matchingLogs.ResourceLogs().MoveAndAppendTo(outLogs.ResourceLogs())

				nonMatchingLogs := createLogData(5,
					pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
						"not.a.match": pdata.NewAttributeValueString("random"),
					}))
				nonMatchingLogs.ResourceLogs().MoveAndAppendTo(outLogs.ResourceLogs())

				return outLogs
			},
			numDroppedLogs: 5,
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

			exp, err := newExporter(tt.config, zap.NewNop())
			require.NoError(t, err)
			require.NotNil(t, exp)
			err = exp.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			numDroppedLogs, err := exp.pushLogData(context.Background(), tt.genLogsFunc())

			if tt.errFunc != nil {
				tt.errFunc(err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.numDroppedLogs, numDroppedLogs)
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
				conventions.AttributeContainerName: "container_name",
				conventions.AttributeK8sCluster:    "k8s_cluster_name",
				"severity":                         "severity",
			},
		},
	}
	exp, err := newExporter(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, exp)
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	t.Run("with attributes that match config", func(t *testing.T) {
		logs := pdata.NewLogs()
		logs.ResourceLogs().Resize(1)
		rl := logs.ResourceLogs().At(0)
		rl.InstrumentationLibraryLogs().Resize(1)
		ill := rl.InstrumentationLibraryLogs().At(0)

		ts := pdata.TimestampUnixNano(int64(1) * time.Millisecond.Nanoseconds())
		lr := pdata.NewLogRecord()
		lr.Body().SetStringVal("log message")
		lr.Attributes().InsertString("not.in.config", "not allowed")
		lr.SetTimestamp(ts)
		ill.Logs().Append(lr)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		expectedPr := &logproto.PushRequest{Streams: make([]logproto.Stream, 0)}
		require.Equal(t, 1, numDroppedLogs)
		require.Equal(t, expectedPr, pr)
	})

	t.Run("with partial attributes that match config", func(t *testing.T) {
		logs := pdata.NewLogs()
		logs.ResourceLogs().Resize(1)
		rl := logs.ResourceLogs().At(0)
		rl.InstrumentationLibraryLogs().Resize(1)
		ill := rl.InstrumentationLibraryLogs().At(0)

		ts := pdata.TimestampUnixNano(int64(1) * time.Millisecond.Nanoseconds())
		lr := pdata.NewLogRecord()
		lr.Body().SetStringVal("log message")
		lr.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lr.Attributes().InsertString("severity", "info")
		lr.Attributes().InsertString("random.attribute", "random")
		lr.SetTimestamp(ts)
		ill.Logs().Append(lr)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 1)
	})

	t.Run("with multiple logs and same attributes", func(t *testing.T) {
		logs := pdata.NewLogs()
		logs.ResourceLogs().Resize(1)
		rl := logs.ResourceLogs().At(0)
		rl.InstrumentationLibraryLogs().Resize(1)
		ill := rl.InstrumentationLibraryLogs().At(0)

		ts := pdata.TimestampUnixNano(int64(1) * time.Millisecond.Nanoseconds())
		lr1 := pdata.NewLogRecord()
		lr1.Body().SetStringVal("log message 1")
		lr1.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lr1.Attributes().InsertString(conventions.AttributeK8sCluster, "mycluster")
		lr1.Attributes().InsertString("severity", "info")
		lr1.SetTimestamp(ts)
		ill.Logs().Append(lr1)

		lr2 := pdata.NewLogRecord()
		lr2.Body().SetStringVal("log message 2")
		lr2.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer")
		lr2.Attributes().InsertString(conventions.AttributeK8sCluster, "mycluster")
		lr2.Attributes().InsertString("severity", "info")
		lr2.SetTimestamp(ts)
		ill.Logs().Append(lr2)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 1)
		require.Len(t, pr.Streams[0].Entries, 2)
	})

	t.Run("with multiple logs and different attributes", func(t *testing.T) {
		logs := pdata.NewLogs()
		logs.ResourceLogs().Resize(1)
		rl := logs.ResourceLogs().At(0)
		rl.InstrumentationLibraryLogs().Resize(1)
		ill := rl.InstrumentationLibraryLogs().At(0)

		ts := pdata.TimestampUnixNano(int64(1) * time.Millisecond.Nanoseconds())
		lr1 := pdata.NewLogRecord()
		lr1.Body().SetStringVal("log message 1")
		lr1.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer1")
		lr1.Attributes().InsertString(conventions.AttributeK8sCluster, "mycluster1")
		lr1.Attributes().InsertString("severity", "debug")
		lr1.SetTimestamp(ts)
		ill.Logs().Append(lr1)

		lr2 := pdata.NewLogRecord()
		lr2.Body().SetStringVal("log message 2")
		lr2.Attributes().InsertString(conventions.AttributeContainerName, "mycontainer2")
		lr2.Attributes().InsertString(conventions.AttributeK8sCluster, "mycluster2")
		lr2.Attributes().InsertString("severity", "error")
		lr2.SetTimestamp(ts)
		ill.Logs().Append(lr2)

		pr, numDroppedLogs := exp.logDataToLoki(logs)
		require.Equal(t, 0, numDroppedLogs)
		require.NotNil(t, pr)
		require.Len(t, pr.Streams, 2)
		require.Len(t, pr.Streams[0].Entries, 1)
		require.Len(t, pr.Streams[1].Entries, 1)
	})
}

func TestExporter_convertAttributesToLabels(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes: map[string]string{
				conventions.AttributeContainerName: "container_name",
				conventions.AttributeK8sCluster:    "k8s_cluster_name",
				"severity":                         "severity",
			},
		},
	}
	exp, err := newExporter(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, exp)
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	t.Run("with attributes that match", func(t *testing.T) {
		am := pdata.NewAttributeMap()
		am.InsertString(conventions.AttributeContainerName, "mycontainer")
		am.InsertString(conventions.AttributeK8sCluster, "mycluster")
		am.InsertString("severity", "debug")

		ls, ok := exp.convertAttributesToLabels(am)
		expLs := model.LabelSet{
			model.LabelName("container_name"):   model.LabelValue("mycontainer"),
			model.LabelName("k8s_cluster_name"): model.LabelValue("mycluster"),
			model.LabelName("severity"):         model.LabelValue("debug"),
		}
		require.True(t, ok)
		require.Equal(t, expLs, ls)
	})

	t.Run("with attribute matches and the value is a boolean", func(t *testing.T) {
		am := pdata.NewAttributeMap()
		am.InsertBool("severity", false)

		ls, ok := exp.convertAttributesToLabels(am)
		require.False(t, ok)
		require.Nil(t, ls)
	})

	t.Run("with attribute that matches and the value is a double", func(t *testing.T) {
		am := pdata.NewAttributeMap()
		am.InsertDouble("severity", float64(0))

		ls, ok := exp.convertAttributesToLabels(am)
		require.False(t, ok)
		require.Nil(t, ls)
	})

	t.Run("with attribute that matches and the value is an int", func(t *testing.T) {
		am := pdata.NewAttributeMap()
		am.InsertInt("severity", 0)

		ls, ok := exp.convertAttributesToLabels(am)
		require.False(t, ok)
		require.Nil(t, ls)
	})

	t.Run("with attribute that matches and the value is null", func(t *testing.T) {
		am := pdata.NewAttributeMap()
		am.InsertNull("severity")

		ls, ok := exp.convertAttributesToLabels(am)
		require.False(t, ok)
		require.Nil(t, ls)
	})
}

func TestExporter_convertLogToLokiEntry(t *testing.T) {
	ts := pdata.TimestampUnixNano(int64(1) * time.Millisecond.Nanoseconds())
	lr := pdata.NewLogRecord()
	lr.Body().SetStringVal("log message")
	lr.SetTimestamp(ts)

	entry := convertLogToLokiEntry(lr)

	expEntry := &logproto.Entry{
		Timestamp: time.Unix(0, int64(lr.Timestamp())),
		Line:      "log message",
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

func TestExporter_startAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes: testValidAttributesWithMapping,
		},
	}
	e, err := newExporter(config, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, e.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporter_stopAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		Labels: LabelsConfig{
			Attributes: testValidAttributesWithMapping,
		},
	}
	e, err := newExporter(config, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, e.stop(context.Background()))
}
