// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkarequest"
)

func TestRequestRetry_ScenarioA_DuplicateDeliveryPrevented(t *testing.T) {
	const (
		topicA = "topic-scenario-a-ack"
		topicC = "topic-scenario-a-retryable"
	)

	config := createDefaultConfig().(*Config)
	config.Producer.MaxMessageBytes = 1024 * 1024
	exp, cluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), topicA, topicC)

	var (
		topicASends atomic.Int32
		topicCSends atomic.Int32
		cHealthy    atomic.Bool
	)

	cluster.ControlKey(new(kmsg.ProduceRequest).Key(), func(req kmsg.Request) (kmsg.Response, error, bool) {
		cluster.KeepControl()
		pr := req.(*kmsg.ProduceRequest)
		resp := kmsg.NewProduceResponse()
		resp.Version = pr.Version
		for _, top := range pr.Topics {
			tr := kmsg.NewProduceResponseTopic()
			tr.Topic = top.Topic
			tr.TopicID = top.TopicID
			topicName := top.Topic
			if topicName == "" {
				if info := cluster.TopicIDInfo(top.TopicID); info != nil {
					topicName = info.Topic
				}
			}
			for _, p := range top.Partitions {
				prt := kmsg.NewProduceResponseTopicPartition()
				prt.Partition = p.Partition
				switch topicName {
				case topicA:
					topicASends.Add(1)
					prt.ErrorCode = 0 // Acknowledged
				case topicC:
					topicCSends.Add(1)
					if cHealthy.Load() {
						prt.ErrorCode = 0 // Succeeded on retry
					} else {
						prt.ErrorCode = kerr.NotEnoughReplicas.Code // Transient failure
					}
				default:
					prt.ErrorCode = 0
				}
				tr.Partitions = append(tr.Partitions, prt)
			}
			resp.Topics = append(resp.Topics, tr)
		}
		return &resp, nil, true
	})

	recA := &kgo.Record{Topic: topicA, Value: []byte("val-a")}
	recC := &kgo.Record{Topic: topicC, Value: []byte("val-c")}
	req := kafkarequest.New([]*kgo.Record{recA, recC})

	pusher := newRequestPusher(exp)

	// Attempt 1: RecA succeeds, RecC fails transiently until attempt deadline expires.
	attempt1Ctx, cancel1 := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel1()
	err := pusher(attempt1Ctx, req)
	require.Error(t, err)
	assert.False(t, consumererror.IsPermanent(err), "transient error must not be marked permanent")
	assert.Equal(t, int32(1), topicASends.Load(), "topicA sent once on attempt 1")
	assert.Positive(t, topicCSends.Load(), "topicC attempted on attempt 1")

	// Helper invokes OnError: RecA must be excluded, only RecC retained.
	retriedReq := req.OnError(err)
	kr, ok := retriedReq.(*kafkarequest.Request)
	require.True(t, ok)
	require.Len(t, kr.Records(), 1, "only retryable records must be retained")
	assert.Same(t, recC, kr.Records()[0])

	// Attempt 2 (Retry): Topic C has recovered.
	cHealthy.Store(true)
	topicCSendsBeforeRetry := topicCSends.Load()
	err = pusher(t.Context(), retriedReq)
	require.NoError(t, err)

	// Final verification: RecA was NOT resent; RecC was retried and succeeded.
	assert.Equal(t, int32(1), topicASends.Load(), "RecA must not be resent after successful acknowledgement")
	assert.Equal(t, topicCSendsBeforeRetry+1, topicCSends.Load(), "RecC must be produced once on retry attempt")
}

func TestRequestRetry_ScenarioB_LostRetryOpportunityPrevented(t *testing.T) {
	const (
		topicA = "topic-scenario-b-ack"
		topicB = "topic-scenario-b-perm"
		topicC = "topic-scenario-b-retryable"
	)

	config := createDefaultConfig().(*Config)
	config.Producer.MaxMessageBytes = 1024 * 1024
	exp, cluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), topicA, topicB, topicC)

	var (
		topicASends atomic.Int32
		topicBSends atomic.Int32
		topicCSends atomic.Int32
		cHealthy    atomic.Bool
	)

	cluster.ControlKey(new(kmsg.ProduceRequest).Key(), func(req kmsg.Request) (kmsg.Response, error, bool) {
		cluster.KeepControl()
		pr := req.(*kmsg.ProduceRequest)
		resp := kmsg.NewProduceResponse()
		resp.Version = pr.Version
		for _, top := range pr.Topics {
			tr := kmsg.NewProduceResponseTopic()
			tr.Topic = top.Topic
			tr.TopicID = top.TopicID
			topicName := top.Topic
			if topicName == "" {
				if info := cluster.TopicIDInfo(top.TopicID); info != nil {
					topicName = info.Topic
				}
			}
			for _, p := range top.Partitions {
				prt := kmsg.NewProduceResponseTopicPartition()
				prt.Partition = p.Partition
				switch topicName {
				case topicA:
					topicASends.Add(1)
					prt.ErrorCode = 0 // Acknowledged
				case topicB:
					topicBSends.Add(1)
					prt.ErrorCode = kerr.MessageTooLarge.Code // Permanent failure
				case topicC:
					topicCSends.Add(1)
					if cHealthy.Load() {
						prt.ErrorCode = 0 // Succeeded on retry
					} else {
						prt.ErrorCode = kerr.NotEnoughReplicas.Code // Transient failure
					}
				default:
					prt.ErrorCode = 0
				}
				tr.Partitions = append(tr.Partitions, prt)
			}
			resp.Topics = append(resp.Topics, tr)
		}
		return &resp, nil, true
	})

	recA := &kgo.Record{Topic: topicA, Value: []byte("val-a")}
	recB := &kgo.Record{Topic: topicB, Value: []byte("val-b")}
	recC := &kgo.Record{Topic: topicC, Value: []byte("val-c")}
	req := kafkarequest.New([]*kgo.Record{recA, recB, recC})

	pusher := newRequestPusher(exp)

	// Attempt 1: RecA succeeds, RecB fails permanently, RecC fails transiently until attempt deadline expires.
	attempt1Ctx, cancel1 := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel1()
	err := pusher(attempt1Ctx, req)
	require.Error(t, err)

	// Critical check: Permanent failure of B must NOT mark the aggregate error permanent,
	// because C remains eligible for retry.
	assert.False(t, consumererror.IsPermanent(err), "mixed batch containing retryable records must not be marked permanent")
	assert.Equal(t, int32(1), topicASends.Load())
	assert.Equal(t, int32(1), topicBSends.Load())
	assert.Positive(t, topicCSends.Load())

	// Helper invokes OnError: RecA (acked) and RecB (permanent) are excluded, only RecC retained.
	retriedReq := req.OnError(err)
	kr, ok := retriedReq.(*kafkarequest.Request)
	require.True(t, ok)
	require.Len(t, kr.Records(), 1, "only retryable record C must be retained")
	assert.Same(t, recC, kr.Records()[0])

	// Attempt 2 (Retry): Topic C recovers.
	cHealthy.Store(true)
	topicCSendsBeforeRetry := topicCSends.Load()
	err = pusher(t.Context(), retriedReq)
	require.NoError(t, err)

	// Final verification: Neither A nor B was resent; C was retried and succeeded.
	assert.Equal(t, int32(1), topicASends.Load(), "RecA must not be resent")
	assert.Equal(t, int32(1), topicBSends.Load(), "RecB must not be resent")
	assert.Equal(t, topicCSendsBeforeRetry+1, topicCSends.Load(), "RecC must be retried")
}

func TestRequestRetry_AllPermanentFailures_MarkedPermanent(t *testing.T) {
	const topicB = "topic-all-perm"

	config := createDefaultConfig().(*Config)
	config.Producer.MaxMessageBytes = 1024 * 1024
	exp, cluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), topicB)

	cluster.ControlKey(new(kmsg.ProduceRequest).Key(), func(req kmsg.Request) (kmsg.Response, error, bool) {
		cluster.KeepControl()
		pr := req.(*kmsg.ProduceRequest)
		resp := kmsg.NewProduceResponse()
		resp.Version = pr.Version
		for _, top := range pr.Topics {
			tr := kmsg.NewProduceResponseTopic()
			tr.Topic = top.Topic
			tr.TopicID = top.TopicID
			for _, p := range top.Partitions {
				prt := kmsg.NewProduceResponseTopicPartition()
				prt.Partition = p.Partition
				prt.ErrorCode = kerr.MessageTooLarge.Code
				tr.Partitions = append(tr.Partitions, prt)
			}
			resp.Topics = append(resp.Topics, tr)
		}
		return &resp, nil, true
	})

	recB := &kgo.Record{Topic: topicB, Value: []byte("val-b")}
	req := kafkarequest.New([]*kgo.Record{recB})

	pusher := newRequestPusher(exp)
	err := pusher(t.Context(), req)
	require.Error(t, err)

	// When all failed records are permanent, the error MUST be marked permanent so helper drops it.
	assert.True(t, consumererror.IsPermanent(err), "all-permanent batch must be classified as permanent")
}
