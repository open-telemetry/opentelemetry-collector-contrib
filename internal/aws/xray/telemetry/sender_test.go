// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

type mockXRayClient struct {
	putTraceSegments    func(ctx context.Context, params *xray.PutTraceSegmentsInput, optFns ...func(*xray.Options)) (*xray.PutTraceSegmentsOutput, error)
	putTelemetryRecords func(ctx context.Context, params *xray.PutTelemetryRecordsInput, optFns ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error)
}

func (m mockXRayClient) PutTraceSegments(ctx context.Context, params *xray.PutTraceSegmentsInput, optFns ...func(*xray.Options)) (*xray.PutTraceSegmentsOutput, error) {
	return m.putTraceSegments(ctx, params, optFns...)
}

func (m mockXRayClient) PutTelemetryRecords(ctx context.Context, params *xray.PutTelemetryRecordsInput, optFns ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error) {
	return m.putTelemetryRecords(ctx, params, optFns...)
}

func TestRotateRace(t *testing.T) {
	var count atomic.Int64
	count.Store(0)
	client := &mockXRayClient{
		putTelemetryRecords: func(_ context.Context, _ *xray.PutTelemetryRecordsInput, _ ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error) {
			count.Add(1)
			if count.Load() >= 2 {
				return nil, errors.New("error")
			}
			return nil, nil
		},
	}
	sender := newSender(client, WithInterval(100*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sender.Start(ctx)
	defer sender.Stop()
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
		return count.Load() >= 1
	}, time.Second, 5*time.Millisecond)
}

func TestIncludeMetadata(t *testing.T) {
	cfg := Config{IncludeMetadata: false}
	awsConfig := aws.Config{}
	set := &awsutil.AWSSessionSettings{ResourceARN: "session_arn"}
	opts := ToOptions(context.Background(), cfg, awsConfig, set)
	assert.Empty(t, opts)
	cfg.IncludeMetadata = true
	opts = ToOptions(context.Background(), cfg, awsConfig, set)
	sender := newSender(&mockXRayClient{}, opts...)
	assert.Empty(t, sender.hostname)
	assert.Empty(t, sender.instanceID)
	assert.Equal(t, "session_arn", sender.resourceARN)
	t.Setenv(envAWSHostname, "env_hostname")
	t.Setenv(envAWSInstanceID, "env_instance_id")
	opts = ToOptions(context.Background(), cfg, awsConfig, &awsutil.AWSSessionSettings{})
	sender = newSender(&mockXRayClient{}, opts...)
	assert.Equal(t, "env_hostname", sender.hostname)
	assert.Equal(t, "env_instance_id", sender.instanceID)
	assert.Empty(t, sender.resourceARN)
	cfg.Hostname = "cfg_hostname"
	cfg.InstanceID = "cfg_instance_id"
	cfg.ResourceARN = "cfg_arn"
	opts = ToOptions(context.Background(), cfg, awsConfig, &awsutil.AWSSessionSettings{})
	sender = newSender(&mockXRayClient{}, opts...)
	assert.Equal(t, "cfg_hostname", sender.hostname)
	assert.Equal(t, "cfg_instance_id", sender.instanceID)
	assert.Equal(t, "cfg_arn", sender.resourceARN)
}

func TestQueueOverflow(t *testing.T) {
	var count atomic.Int64
	count.Store(0)
	obs, logs := observer.New(zap.DebugLevel)
	client := &mockXRayClient{
		putTelemetryRecords: func(_ context.Context, _ *xray.PutTelemetryRecordsInput, _ ...func(*xray.Options)) (*xray.PutTelemetryRecordsOutput, error) {
			count.Add(1)
			if count.Load() >= 2 {
				return nil, errors.New("error")
			}
			return nil, nil
		},
	}
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
	assert.Len(t, sender.queue, 20)
	sender.send(context.Background())
	// only one batch succeeded
	assert.Len(t, sender.queue, 15)
	// verify that sent back of queue
	for _, record := range sender.queue {
		assert.Greater(t, *record.SegmentsSentCount, int32(5))
		assert.LessOrEqual(t, *record.SegmentsSentCount, int32(20))
	}
}
