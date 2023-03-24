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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

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
	mc := &mockClient{count: &atomic.Int64{}}
	mc.On("PutTelemetryRecords", mock.Anything).Return(nil, nil).Once()
	mc.On("PutTelemetryRecords", mock.Anything).Return(nil, errors.New("error"))
	recorder := newTelemetryRecorder(mc)
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
	cfg := Config{IncludeMetadata: false}
	sess := awsmock.Session
	set := &awsutil.AWSSessionSettings{ResourceARN: "session_arn"}
	opts := ToRecorderOptions(cfg, sess, set)
	assert.Empty(t, opts)
	cfg.IncludeMetadata = true
	opts = ToRecorderOptions(cfg, sess, set)
	recorder := newTelemetryRecorder(&mockClient{}, opts...)
	assert.Equal(t, "", recorder.hostname)
	assert.Equal(t, "", recorder.instanceID)
	assert.Equal(t, "session_arn", recorder.resourceARN)
	t.Setenv(envAWSHostname, "env_hostname")
	t.Setenv(envAWSInstanceID, "env_instance_id")
	opts = ToRecorderOptions(cfg, sess, &awsutil.AWSSessionSettings{})
	recorder = newTelemetryRecorder(&mockClient{}, opts...)
	assert.Equal(t, "env_hostname", recorder.hostname)
	assert.Equal(t, "env_instance_id", recorder.instanceID)
	assert.Equal(t, "", recorder.resourceARN)
	cfg.Hostname = "cfg_hostname"
	cfg.InstanceID = "cfg_instance_id"
	cfg.ResourceARN = "cfg_arn"
	opts = ToRecorderOptions(cfg, sess, &awsutil.AWSSessionSettings{})
	recorder = newTelemetryRecorder(&mockClient{}, opts...)
	assert.Equal(t, "cfg_hostname", recorder.hostname)
	assert.Equal(t, "cfg_instance_id", recorder.instanceID)
	assert.Equal(t, "cfg_arn", recorder.resourceARN)
}

func TestRecordConnectionError(t *testing.T) {
	type testParameters struct {
		input error
		want  func() *xray.TelemetryRecord
	}
	testCases := []testParameters{
		{
			input: awserr.NewRequestFailure(nil, http.StatusInternalServerError, ""),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.HTTPCode5XXCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.NewRequestFailure(nil, http.StatusBadRequest, ""),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.HTTPCode4XXCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.NewRequestFailure(nil, http.StatusFound, ""),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeResponseTimeout, "", nil),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.TimeoutCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeRequestError, "", nil),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.UnknownHostCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeSerialization, "", nil),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int64(1)
				return record
			},
		},
		{
			input: errors.New("test"),
			want: func() *xray.TelemetryRecord {
				record := newTelemetryRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int64(1)
				return record
			},
		},
		{
			input: nil,
			want:  newTelemetryRecord,
		},
	}
	recorder := newTelemetryRecorder(&mockClient{})
	for _, testCase := range testCases {
		recorder.RecordConnectionError(testCase.input)
		snapshot := recorder.cutoff()
		assert.EqualValues(t, testCase.want().BackendConnectionErrors, snapshot.BackendConnectionErrors)
	}
}

func TestQueueOverflow(t *testing.T) {
	obs, logs := observer.New(zap.DebugLevel)
	recorder := newTelemetryRecorder(&mockClient{}, WithLogger(zap.New(obs)))
	for i := 1; i <= queueSize+20; i++ {
		recorder.RecordSegmentsSent(i)
		recorder.add(recorder.cutoff())
	}
	assert.Equal(t, 20, logs.Len())
	recorder.Stop()
	recorder.add(recorder.cutoff())
}
