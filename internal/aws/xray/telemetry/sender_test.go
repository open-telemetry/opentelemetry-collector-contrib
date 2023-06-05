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
	client := &mockClient{count: &atomic.Int64{}}
	client.On("PutTelemetryRecords", mock.Anything).Return(nil, nil).Once()
	client.On("PutTelemetryRecords", mock.Anything).Return(nil, errors.New("error"))
	sender := newSender(client, WithInterval(100*time.Millisecond))
	sender.Start()
	defer sender.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(time.Millisecond)
		for {
			select {
			case <-ticker.C:
				sender.RecordSegmentsReceived(1)
				sender.RecordSegmentsSpillover(1)
				sender.RecordSegmentsRejected(1)
			case <-ctx.Done():
				return
			}
		}
	}()
	assert.Eventually(t, func() bool {
		return client.count.Load() >= 2
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
	sender := newSender(&mockClient{}, opts...)
	assert.Equal(t, "", sender.hostname)
	assert.Equal(t, "", sender.instanceID)
	assert.Equal(t, "session_arn", sender.resourceARN)
	t.Setenv(envAWSHostname, "env_hostname")
	t.Setenv(envAWSInstanceID, "env_instance_id")
	opts = ToOptions(cfg, sess, &awsutil.AWSSessionSettings{})
	sender = newSender(&mockClient{}, opts...)
	assert.Equal(t, "env_hostname", sender.hostname)
	assert.Equal(t, "env_instance_id", sender.instanceID)
	assert.Equal(t, "", sender.resourceARN)
	cfg.Hostname = "cfg_hostname"
	cfg.InstanceID = "cfg_instance_id"
	cfg.ResourceARN = "cfg_arn"
	opts = ToOptions(cfg, sess, &awsutil.AWSSessionSettings{})
	sender = newSender(&mockClient{}, opts...)
	assert.Equal(t, "cfg_hostname", sender.hostname)
	assert.Equal(t, "cfg_instance_id", sender.instanceID)
	assert.Equal(t, "cfg_arn", sender.resourceARN)
}

func TestQueueOverflow(t *testing.T) {
	obs, logs := observer.New(zap.DebugLevel)
	client := &mockClient{count: &atomic.Int64{}}
	client.On("PutTelemetryRecords", mock.Anything).Return(nil, nil).Once()
	client.On("PutTelemetryRecords", mock.Anything).Return(nil, errors.New("test"))
	sender := newSender(
		client,
		WithLogger(zap.New(obs)),
		WithInterval(time.Millisecond),
		WithQueueSize(20),
		WithBatchSize(5),
	)
	for i := 1; i <= 25; i++ {
		sender.RecordSegmentsSent(i)
		sender.enqueue(sender.Rotate())
	}
	// number of dropped records
	assert.Equal(t, 5, logs.Len())
	assert.Equal(t, 20, len(sender.queue))
	sender.send()
	// only one batch succeeded
	assert.Equal(t, 15, len(sender.queue))
	// verify that sent back of queue
	for _, record := range sender.queue {
		assert.Greater(t, *record.SegmentsSentCount, int64(5))
		assert.LessOrEqual(t, *record.SegmentsSentCount, int64(20))
	}
}
