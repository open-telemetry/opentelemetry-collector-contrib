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
	"sync/atomic"
	"testing"
	"time"

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

func TestRotateRace(t *testing.T) {
	mc := &mockClient{count: &atomic.Int64{}}
	mc.On("PutTelemetryRecords", mock.Anything).Return(nil, nil).Once()
	mc.On("PutTelemetryRecords", mock.Anything).Return(nil, errors.New("error"))
	recorder := newSender(mc, WithInterval(50*time.Millisecond))
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
	opts := ToOptions(cfg, sess, set)
	assert.Empty(t, opts)
	cfg.IncludeMetadata = true
	opts = ToOptions(cfg, sess, set)
	recorder := newSender(&mockClient{}, opts...)
	assert.Equal(t, "", recorder.hostname)
	assert.Equal(t, "", recorder.instanceID)
	assert.Equal(t, "session_arn", recorder.resourceARN)
	t.Setenv(envAWSHostname, "env_hostname")
	t.Setenv(envAWSInstanceID, "env_instance_id")
	opts = ToOptions(cfg, sess, &awsutil.AWSSessionSettings{})
	recorder = newSender(&mockClient{}, opts...)
	assert.Equal(t, "env_hostname", recorder.hostname)
	assert.Equal(t, "env_instance_id", recorder.instanceID)
	assert.Equal(t, "", recorder.resourceARN)
	cfg.Hostname = "cfg_hostname"
	cfg.InstanceID = "cfg_instance_id"
	cfg.ResourceARN = "cfg_arn"
	opts = ToOptions(cfg, sess, &awsutil.AWSSessionSettings{})
	recorder = newSender(&mockClient{}, opts...)
	assert.Equal(t, "cfg_hostname", recorder.hostname)
	assert.Equal(t, "cfg_instance_id", recorder.instanceID)
	assert.Equal(t, "cfg_arn", recorder.resourceARN)
}

func TestQueueOverflow(t *testing.T) {
	obs, logs := observer.New(zap.DebugLevel)
	recorder := newSender(&mockClient{}, WithLogger(zap.New(obs)))
	for i := 1; i <= defaultQueueSize+20; i++ {
		recorder.RecordSegmentsSent(i)
		recorder.add(recorder.Rotate())
	}
	assert.Equal(t, 20, logs.Len())
	recorder.Stop()
	recorder.add(recorder.Rotate())
}
