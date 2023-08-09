// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

type wantedBody struct {
	Name       string
	Time       float64
	Attributes map[string]string
	Value      float64
}

var endpoint = "localhost:8081"

type testCase struct {
	Name         string
	HTTPMethod   string
	QueryParams  string
	RequestBody  string
	ResponseCode int
	WantData     []pmetric.Metrics
}

func TestNewReceiver(t *testing.T) {
	type args struct {
		addr         string
		timeout      time.Duration
		attrsPrefix  string
		nextConsumer consumer.Metrics
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "nil next Consumer",
			args: args{
				addr:        ":0",
				timeout:     defaultTimeout,
				attrsPrefix: "default_attr_",
			},
			wantErr: component.ErrNilNextConsumer,
		},
		{
			name: "happy path",
			args: args{
				addr:         ":0",
				timeout:      defaultTimeout,
				attrsPrefix:  "default_attr_",
				nextConsumer: consumertest.NewNop(),
			},
		},
	}
	logger := zap.NewNop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newCollectdReceiver(logger, tt.args.addr, time.Second*10, "", tt.args.nextConsumer)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestCollectDServer(t *testing.T) {
	t.Parallel()
	defaultAttrsPrefix := "dap_"

	wantedRequestBody := wantedBody{
		Name: "memory.free",
		Time: 1415062577.4949999,
		Attributes: map[string]string{
			"plugin": "memory",
			"host":   "i-b13d1e5f",
			"dsname": "value",
			"attr1":  "attr1val",
		},
		Value: 2.1474,
	}
	wantedRequestBodyMetrics := createWantedMetrics(wantedRequestBody)

	testInvalidHTTPMethodCase := testCase{
		Name:         "invalid-http-method",
		HTTPMethod:   "GET",
		RequestBody:  `invalid-body`,
		ResponseCode: 400,
		WantData:     []pmetric.Metrics{},
	}

	testValidRequestBodyCase := testCase{
		Name:        "valid-request-body",
		HTTPMethod:  "POST",
		QueryParams: "dap_attr1=attr1val",
		RequestBody: `[
    	{
			"dsnames": [
				"value"
			],
			"dstypes": [
				"derive"
			],
			"host": "i-b13d1e5f",
			"interval": 10.0,
			"plugin": "memory",
			"plugin_instance": "",
			"time": 1415062577.4949999,
			"type": "memory",
			"type_instance": "free",
			"values": [
				2.1474
			]
		}
	]`,
		ResponseCode: 200,
		WantData:     []pmetric.Metrics{wantedRequestBodyMetrics},
	}

	testInValidRequestBodyCase := testCase{
		Name:         "invalid-request-body",
		HTTPMethod:   "POST",
		RequestBody:  `invalid-body`,
		ResponseCode: 400,
		WantData:     []pmetric.Metrics{},
	}

	testCases := []testCase{testInvalidHTTPMethodCase, testValidRequestBodyCase, testInValidRequestBodyCase}

	sink := new(consumertest.MetricsSink)

	logger := zap.NewNop()
	cdr, err := newCollectdReceiver(logger, endpoint, defaultTimeout, defaultAttrsPrefix, sink)
	if err != nil {
		t.Fatalf("Failed to create receiver: %v", err)
	}

	require.NoError(t, cdr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		err := cdr.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Error stopping metrics reception: %v", err)
		}
	}()

	time.Sleep(time.Second)

	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			sink.Reset()
			req, err := http.NewRequest(
				tt.HTTPMethod,
				"http://"+endpoint+"?"+tt.QueryParams,
				bytes.NewBuffer([]byte(tt.RequestBody)),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, tt.ResponseCode, resp.StatusCode)
			defer resp.Body.Close()

			if tt.ResponseCode != 200 {
				return
			}

			assert.Eventually(t, func() bool {
				return len(sink.AllMetrics()) == 1
			}, 10*time.Second, 5*time.Millisecond)
			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			assertMetricsAreEqual(t, tt.WantData, mds)
		})
	}
}

func createWantedMetrics(wantedBody wantedBody) pmetric.Metrics {
	var dataPoint pmetric.NumberDataPoint
	testMetrics := pmetric.NewMetrics()
	scopeMemtrics := testMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	testMetric := pmetric.NewMetric()
	testMetric.SetName(wantedBody.Name)
	sum := testMetric.SetEmptySum()
	sum.SetIsMonotonic(true)
	dataPoint = sum.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(float64(time.Second)*wantedBody.Time))))
	attributes := pcommon.NewMap()

	for key, value := range wantedBody.Attributes {
		attributes.PutStr(key, value)
	}

	attributes.CopyTo(dataPoint.Attributes())
	dataPoint.SetDoubleValue(wantedBody.Value)

	newMetric := scopeMemtrics.Metrics().AppendEmpty()
	testMetric.MoveTo(newMetric)
	return testMetrics
}

func assertMetricsAreEqual(t *testing.T, expectedData []pmetric.Metrics, actualData []pmetric.Metrics) {

	for i := 0; i < len(expectedData); i++ {
		err := pmetrictest.CompareMetrics(expectedData[i], actualData[i])
		require.NoError(t, err)
	}
}
