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

package telemetry

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

type mockClient struct {
	mock.Mock
	count *atomic.Int64
}

func (m *mockClient) PutTraceSegments(input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*xray.PutTraceSegmentsOutput), args.Error(1)
}

func (m *mockClient) PutTelemetryRecords(input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error) {
	args := m.Called(input)
	m.count.Add(1)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*xray.PutTelemetryRecordsOutput), args.Error(1)
}

func TestCutoffInterval(t *testing.T) {
	registry := NewRegistry()
	mc := &mockClient{count: &atomic.Int64{}}
	mc.On("PutTelemetryRecords", mock.Anything).Return(nil, nil).Once()
	mc.On("PutTelemetryRecords", mock.Anything).Return(nil, errors.New("error"))
	recorder := registry.Register(
		component.NewID("test"),
		mc,
		nil,
		&Config{IncludeMetadata: false},
		nil,
	).(*telemetryRecorder)
	recorder.interval = 50 * time.Millisecond
	recorder.Start()
	defer recorder.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		for {
			select {
			case <-ticker.C:
				recorder.RecordSegmentsReceived(1)
				recorder.RecordSegmentsSpillover(1)
				recorder.RecordSegmentsRejected(1)
			case <-ctx.Done():
				return
			}
		}
	}()
	assert.Eventually(t, func() bool {
		return mc.count.Load() >= 2
	}, time.Second, 5*time.Millisecond)
}

func TestIncludeMetadata(t *testing.T) {
	sess := awsmock.Session
	recorder := newTelemetryRecorder(
		nil,
		sess,
		&Config{IncludeMetadata: true},
		&awsutil.AWSSessionSettings{ResourceARN: "session_arn"},
	).(*telemetryRecorder)
	assert.Equal(t, "", recorder.hostname)
	assert.Equal(t, "", recorder.instanceID)
	assert.Equal(t, "session_arn", recorder.resourceARN)

	t.Setenv(envAWSHostname, "env_hostname")
	t.Setenv(envAWSInstanceID, "env_instance_id")
	recorder = newTelemetryRecorder(
		nil,
		sess,
		&Config{IncludeMetadata: true},
		nil,
	).(*telemetryRecorder)
	assert.Equal(t, "env_hostname", recorder.hostname)
	assert.Equal(t, "env_instance_id", recorder.instanceID)
	assert.Equal(t, "", recorder.resourceARN)
}

func TestRecordConnectionError(t *testing.T) {
	type testParameters struct {
		errorCode       string
		errorStatusCode int
		field           func(connectionErrors *xray.BackendConnectionErrors) *int64
	}
	testCases := []testParameters{
		{
			errorStatusCode: http.StatusInternalServerError,
			field: func(connectionErrors *xray.BackendConnectionErrors) *int64 {
				return connectionErrors.HTTPCode5XXCount
			},
		},
		{
			errorStatusCode: http.StatusBadRequest,
			field: func(connectionErrors *xray.BackendConnectionErrors) *int64 {
				return connectionErrors.HTTPCode4XXCount
			},
		},
		{
			errorStatusCode: http.StatusFound,
			field: func(connectionErrors *xray.BackendConnectionErrors) *int64 {
				return connectionErrors.OtherCount
			},
		},
		{
			errorCode: request.ErrCodeResponseTimeout,
			field: func(connectionErrors *xray.BackendConnectionErrors) *int64 {
				return connectionErrors.TimeoutCount
			},
		},
		{
			errorCode: request.ErrCodeRequestError,
			field: func(connectionErrors *xray.BackendConnectionErrors) *int64 {
				return connectionErrors.UnknownHostCount
			},
		},
		{
			errorCode: request.ErrCodeSerialization,
			field: func(connectionErrors *xray.BackendConnectionErrors) *int64 {
				return connectionErrors.OtherCount
			},
		},
	}
	recorder := newTelemetryRecorder(
		nil,
		nil,
		nil,
		nil,
	).(*telemetryRecorder)
	origError := errors.New("error")
	for _, testCase := range testCases {
		err := awserr.New(testCase.errorCode, "message", origError)
		if testCase.errorStatusCode != 0 {
			err = awserr.NewRequestFailure(err, testCase.errorStatusCode, "id")
		}
		recorder.RecordConnectionError(err)
		snapshot := recorder.cutoff()
		assert.EqualValues(t, 1, *testCase.field(snapshot.BackendConnectionErrors))
	}
	recorder.RecordConnectionError(origError)
	assert.EqualValues(t, 1, *recorder.record.BackendConnectionErrors.OtherCount)
}

func TestQueueOverflow(t *testing.T) {
	recorder := newTelemetryRecorder(
		nil,
		nil,
		&Config{IncludeMetadata: false},
		nil,
	).(*telemetryRecorder)
	for i := 1; i <= queueSize+20; i++ {
		recorder.RecordSegmentsSent(i)
		recorder.add(recorder.cutoff())
	}
	for len(recorder.queue) > 0 {
		record := <-recorder.queue
		assert.Greater(t, *record.SegmentsSentCount, int64(20))
	}
}
