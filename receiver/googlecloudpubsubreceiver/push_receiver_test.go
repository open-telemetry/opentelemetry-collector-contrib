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

package googlecloudpubsubreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/testdata"
)

type pushTester struct {
	client   http.Client
	endpoint string
	url      string
}

func (c *pushTester) push(t *testing.T, attributes map[string]string, data []byte) *http.Response {
	response, err := c.pushNoAssert(attributes, data)
	assert.NoError(t, err)
	assert.Equal(t, 202, response.StatusCode)
	return response
}

func (c *pushTester) pushNoAssert(attributes map[string]string, data []byte) (*http.Response, error) {
	s, _ := json.Marshal(&PubSubMessage{
		Message: Message{
			Attributes:  attributes,
			Data:        data,
			PublishTime: "2021-02-26T19:13:55.749Z",
			ID:          "2070443601311540",
		},
		Subscription: "projects/my-project/topics/otlp",
	})
	req, _ := http.NewRequest("POST",
		c.url,
		bytes.NewReader(s))
	return c.client.Do(req)
}

func newPushTester(t *testing.T) *pushTester {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		require.NoError(t, err)
		return nil
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		require.NoError(t, err)
		return nil
	}
	defer l.Close()
	return &pushTester{
		client:   http.Client{},
		endpoint: fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port),
		url:      fmt.Sprintf("http://localhost:%d/", l.Addr().(*net.TCPAddr).Port),
	}

}

func shutdown(ctx context.Context, receiver component.Component) {
	_ = receiver.Shutdown(ctx)
}

func TestPushReceiver(t *testing.T) {
	ctx := context.Background()
	tester := newPushTester(t)

	core, _ := observer.New(zap.WarnLevel)
	params := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)
	metricSink := new(consumertest.MetricsSink)
	logSink := new(consumertest.LogsSink)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(typeStr),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: params,
	})
	require.NoError(t, err)

	receiver := &pubsubPushReceiver{
		pubsubReceiver: &pubsubReceiver{
			logger:  zap.New(core),
			obsrecv: obsrecv,
			config: &Config{
				Mode: "push",
				Push: PushConfig{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tester.endpoint,
					},
				},
			},
			tracesConsumer:  traceSink,
			metricsConsumer: metricSink,
			logsConsumer:    logSink,
		},
	}
	assert.NoError(t, receiver.pubsubReceiver.config.validate())
	assert.NoError(t, receiver.Start(ctx, nil))
	defer shutdown(ctx, receiver)

	// Test an OTLP trace message
	traceSink.Reset()

	_ = tester.push(t, map[string]string{
		"ce-type":      "org.opentelemetry.otlp.traces.v1",
		"content-type": "application/protobuf",
	}, testdata.CreateTraceExport())
	assert.Eventually(t, func() bool {
		return len(traceSink.AllTraces()) == 1
	}, 100*time.Second, 10*time.Millisecond)

	// Test an OTLP metric message
	metricSink.Reset()

	_ = tester.push(t, map[string]string{
		"ce-type":      "org.opentelemetry.otlp.metrics.v1",
		"content-type": "application/protobuf",
	}, testdata.CreateMetricExport())
	assert.Eventually(t, func() bool {
		return len(metricSink.AllMetrics()) == 1
	}, time.Second, 10*time.Millisecond)

	// Test an OTLP log message
	logSink.Reset()
	_ = tester.push(t, map[string]string{
		"ce-type":      "org.opentelemetry.otlp.logs.v1",
		"content-type": "application/protobuf",
	}, testdata.CreateLogExport())
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)

	// Test a plain log message
	logSink.Reset()
	_ = tester.push(t, map[string]string{
		"content-type": "text/plain",
	}, []byte("some text"))
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)

	// Test an GZipped OTLP log message
	logSink.Reset()
	_ = tester.push(t, map[string]string{
		"ce-type":          "org.opentelemetry.otlp.logs.v1",
		"content-type":     "application/protobuf",
		"content-encoding": "gzip",
	}, testdata.CreateGZipped(testdata.CreateLogExport()))
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, time.Second, 10*time.Millisecond)

	assert.Nil(t, receiver.Shutdown(ctx))
	assert.Nil(t, receiver.Shutdown(ctx))
}

func TestPushExplicitCompressedTraces(t *testing.T) {
	ctx := context.Background()
	tester := newPushTester(t)

	core, _ := observer.New(zap.WarnLevel)
	params := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(typeStr),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: params,
	})
	require.NoError(t, err)

	receiver := &pubsubPushReceiver{
		pubsubReceiver: &pubsubReceiver{
			logger:  zap.New(core),
			obsrecv: obsrecv,
			config: &Config{
				Mode:        "push",
				Encoding:    "otlp_proto_trace",
				Compression: "gzip",
				Push: PushConfig{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tester.endpoint,
					},
				},
			},
			tracesConsumer: traceSink,
		},
	}
	assert.NoError(t, receiver.pubsubReceiver.config.validate())
	assert.NoError(t, receiver.Start(ctx, nil))
	defer shutdown(ctx, receiver)

	_ = tester.push(t, map[string]string{}, testdata.CreateGZipped(testdata.CreateTraceExport()))
	assert.Eventually(t, func() bool {
		return len(traceSink.AllTraces()) == 1
	}, 100*time.Second, 10*time.Millisecond)
}

func TestPushExplicitCompressedMetrics(t *testing.T) {
	ctx := context.Background()
	tester := newPushTester(t)

	core, _ := observer.New(zap.WarnLevel)
	params := receivertest.NewNopCreateSettings()
	metricSink := new(consumertest.MetricsSink)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(typeStr),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: params,
	})
	require.NoError(t, err)

	receiver := &pubsubPushReceiver{
		pubsubReceiver: &pubsubReceiver{
			logger:  zap.New(core),
			obsrecv: obsrecv,
			config: &Config{
				Mode:        "push",
				Encoding:    "otlp_proto_metric",
				Compression: "gzip",
				Push: PushConfig{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tester.endpoint,
					},
				},
			},
			metricsConsumer: metricSink,
		},
	}
	assert.NoError(t, receiver.pubsubReceiver.config.validate())
	assert.NoError(t, receiver.Start(ctx, nil))
	defer shutdown(ctx, receiver)

	// Test an OTLP trace message
	_ = tester.push(t, map[string]string{}, testdata.CreateGZipped(testdata.CreateMetricExport()))
	assert.Eventually(t, func() bool {
		return len(metricSink.AllMetrics()) == 1
	}, 100*time.Second, 10*time.Millisecond)
}

func TestPushExplicitCompressedLogs(t *testing.T) {
	ctx := context.Background()
	tester := newPushTester(t)

	core, _ := observer.New(zap.WarnLevel)
	params := receivertest.NewNopCreateSettings()
	logSink := new(consumertest.LogsSink)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(typeStr),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: params,
	})
	require.NoError(t, err)

	receiver := &pubsubPushReceiver{
		pubsubReceiver: &pubsubReceiver{
			logger:  zap.New(core),
			obsrecv: obsrecv,
			config: &Config{
				Mode:        "push",
				Encoding:    "otlp_proto_log",
				Compression: "gzip",
				Push: PushConfig{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tester.endpoint,
					},
				},
			},
			logsConsumer: logSink,
		},
	}
	assert.NoError(t, receiver.pubsubReceiver.config.validate())
	assert.NoError(t, receiver.Start(ctx, nil))
	defer shutdown(ctx, receiver)

	_ = tester.push(t, map[string]string{}, testdata.CreateGZipped(testdata.CreateLogExport()))
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, 100*time.Second, 10*time.Millisecond)
}

func TestPushExplicitCompressedRawText(t *testing.T) {
	ctx := context.Background()
	tester := newPushTester(t)

	core, _ := observer.New(zap.WarnLevel)
	params := receivertest.NewNopCreateSettings()
	logSink := new(consumertest.LogsSink)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(typeStr),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: params,
	})
	require.NoError(t, err)

	receiver := &pubsubPushReceiver{
		pubsubReceiver: &pubsubReceiver{
			logger:  zap.New(core),
			obsrecv: obsrecv,
			config: &Config{
				Mode:        "push",
				Encoding:    "raw_text",
				Compression: "gzip",
				Push: PushConfig{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tester.endpoint,
					},
				},
			},
			logsConsumer: logSink,
		},
	}
	assert.NoError(t, receiver.pubsubReceiver.config.validate())
	assert.NoError(t, receiver.Start(ctx, nil))
	defer shutdown(ctx, receiver)

	_ = tester.push(t, map[string]string{}, testdata.CreateGZipped(testdata.CreateTextExport()))
	assert.Eventually(t, func() bool {
		return len(logSink.AllLogs()) == 1
	}, 100*time.Second, 10*time.Millisecond)
}

func TestPushGarbage(t *testing.T) {
	ctx := context.Background()
	tester := newPushTester(t)

	core, _ := observer.New(zap.WarnLevel)
	params := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)
	metricSink := new(consumertest.MetricsSink)
	logSink := new(consumertest.LogsSink)

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID(typeStr),
		Transport:              reportTransport,
		LongLivedCtx:           false,
		ReceiverCreateSettings: params,
	})
	require.NoError(t, err)

	receiver := &pubsubPushReceiver{
		pubsubReceiver: &pubsubReceiver{
			logger:  zap.New(core),
			obsrecv: obsrecv,
			config: &Config{
				Mode: "push",
				Push: PushConfig{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: tester.endpoint,
					},
				},
			},
			metricsConsumer: metricSink,
			tracesConsumer:  traceSink,
			logsConsumer:    logSink,
		},
	}
	assert.NoError(t, receiver.pubsubReceiver.config.validate())
	assert.NoError(t, receiver.Start(ctx, nil))
	defer shutdown(ctx, receiver)

	response, _ := tester.pushNoAssert(map[string]string{}, testdata.CreateGarbageBytes())
	assert.Equal(t, 400, response.StatusCode)

	response, _ = tester.pushNoAssert(map[string]string{
		"ce-type":      "org.opentelemetry.otlp.traces.v1",
		"content-type": "application/protobuf",
	}, testdata.CreateGarbageBytes())
	assert.Equal(t, 400, response.StatusCode)
}
