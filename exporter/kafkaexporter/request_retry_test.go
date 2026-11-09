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
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkarequest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
)

func newTestMockTracesExporter(t *testing.T, cfg Config, host component.Host, topics ...string) (*kafkaExporter[ptrace.Traces], *kfake.Cluster) {
	cluster, kcfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, topics...))
	kgoClientOpts := []kgo.Opt{
		kgo.SeedBrokers(kcfg.Brokers...),
		kgo.ClientID(cfg.ClientConfig.ClientID),
		kgo.RecordRetries(0), // return retryable error directly without internal loop
		kgo.WithHooks(kafkaclient.NewStatusReporter(host)),
	}

	client, err := kafka.NewFranzSyncProducer(t.Context(), host, kcfg,
		cfg.Producer, 1*time.Second, zap.NewNop(), kgoClientOpts...)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	exp := newTracesExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	messenger, err := exp.newMessenger(host)
	require.NoError(t, err)

	exp.messenger = messenger
	exp.producer = kafkaclient.NewFranzSyncProducer(
		client,
		exp.cfg.IncludeMetadataKeys,
		exp.cfg.RecordHeaders,
		exp.cfg.Producer.MaxMessageBytes,
		nil,
	)
	return exp, cluster
}

func TestRequestRetry_ScenarioA_DuplicateDeliveryPrevented(t *testing.T) {
	const (
		topicA = "topic-scenario-a-ack"
		topicC = "topic-scenario-a-retryable"
	)

	config := createDefaultConfig().(*Config)
	config.Producer.MaxMessageBytes = 1024 * 1024
	exp, cluster := newTestMockTracesExporter(t, *config, componenttest.NewNopHost(), topicA, topicC)

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

	// Attempt 1: RecA succeeds, RecC fails transiently.
	err := pusher(t.Context(), req)
	require.Error(t, err)
	assert.False(t, consumererror.IsPermanent(err), "transient error must not be marked permanent")
	assert.Equal(t, int32(1), topicASends.Load(), "topicA sent once on attempt 1")
	assert.Equal(t, int32(1), topicCSends.Load(), "topicC attempted once on attempt 1")

	// Helper invokes OnError: RecA must be excluded, only RecC retained.
	retriedReq := req.OnError(err)
	kr, ok := retriedReq.(*kafkarequest.Request)
	require.True(t, ok)
	require.Len(t, kr.Records(), 1, "only retryable records must be retained")
	assert.Same(t, recC, kr.Records()[0])

	// Attempt 2 (Retry): Topic C has recovered.
	cHealthy.Store(true)
	err = pusher(t.Context(), retriedReq)
	require.NoError(t, err)

	// Final verification: RecA was NOT resent; RecC was retried and succeeded.
	assert.Equal(t, int32(1), topicASends.Load(), "RecA must not be resent after successful acknowledgement")
	assert.Equal(t, int32(2), topicCSends.Load(), "RecC must be produced once on retry attempt")
}

func TestRequestRetry_ScenarioB_LostRetryOpportunityPrevented(t *testing.T) {
	const (
		topicA = "topic-scenario-b-ack"
		topicB = "topic-scenario-b-perm"
		topicC = "topic-scenario-b-retryable"
	)

	config := createDefaultConfig().(*Config)
	config.Producer.MaxMessageBytes = 1024 * 1024
	exp, cluster := newTestMockTracesExporter(t, *config, componenttest.NewNopHost(), topicA, topicB, topicC)

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

	// Attempt 1: RecA succeeds, RecB fails permanently, RecC fails transiently.
	err := pusher(t.Context(), req)
	require.Error(t, err)

	// Critical check: Permanent failure of B must NOT mark the aggregate error permanent,
	// because C remains eligible for retry.
	assert.False(t, consumererror.IsPermanent(err), "mixed batch containing retryable records must not be marked permanent")
	assert.Equal(t, int32(1), topicASends.Load())
	assert.Equal(t, int32(1), topicBSends.Load())
	assert.Equal(t, int32(1), topicCSends.Load())

	// Helper invokes OnError: RecA (acked) and RecB (permanent) are excluded, only RecC retained.
	retriedReq := req.OnError(err)
	kr, ok := retriedReq.(*kafkarequest.Request)
	require.True(t, ok)
	require.Len(t, kr.Records(), 1, "only retryable record C must be retained")
	assert.Same(t, recC, kr.Records()[0])

	// Attempt 2 (Retry): Topic C recovers.
	cHealthy.Store(true)
	err = pusher(t.Context(), retriedReq)
	require.NoError(t, err)

	// Final verification: Neither A nor B was resent; C was retried and succeeded.
	assert.Equal(t, int32(1), topicASends.Load(), "RecA must not be resent")
	assert.Equal(t, int32(1), topicBSends.Load(), "RecB must not be resent")
	assert.Equal(t, int32(2), topicCSends.Load(), "RecC must be retried")
}

func TestRequestRetry_AllPermanentFailures_MarkedPermanent(t *testing.T) {
	const topicB = "topic-all-perm"

	config := createDefaultConfig().(*Config)
	config.Producer.MaxMessageBytes = 1024 * 1024
	exp, cluster := newTestMockTracesExporter(t, *config, componenttest.NewNopHost(), topicB)

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

func TestExporter_RetryOnFailure_MixedBatch_GateOnAndOff(t *testing.T) {
	const (
		topicA = "topic-real-a-ack"
		topicB = "topic-real-b-perm"
		topicC = "topic-real-c-retry"
	)

	createTraces := func() ptrace.Traces {
		td := ptrace.NewTraces()
		rsA := td.ResourceSpans().AppendEmpty()
		rsA.Resource().Attributes().PutStr("kafka.topic", topicA)
		rsA.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("span-a")

		rsB := td.ResourceSpans().AppendEmpty()
		rsB.Resource().Attributes().PutStr("kafka.topic", topicB)
		rsB.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("span-b")

		rsC := td.ResourceSpans().AppendEmpty()
		rsC.Resource().Attributes().PutStr("kafka.topic", topicC)
		rsC.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("span-c")
		return td
	}

	type exporterTestOpts struct {
		gateEnabled  bool
		retryEnabled bool
		cAlwaysFails bool
	}

	setupExporter := func(t *testing.T, opts exporterTestOpts) (exporter.Traces, *atomic.Int32, *atomic.Int32, *atomic.Int32) {
		setUseRequestTypeFeatureGate(t, opts.gateEnabled)

		cluster, clientConfig := kafkatest.NewCluster(t, kfake.SeedTopics(1, topicA, topicB, topicC))
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
						prt.ErrorCode = 0
					case topicB:
						topicBSends.Add(1)
						prt.ErrorCode = kerr.MessageTooLarge.Code
					case topicC:
						topicCSends.Add(1)
						if cHealthy.Load() {
							prt.ErrorCode = 0
						} else {
							prt.ErrorCode = kerr.NotEnoughReplicas.Code
							if !opts.cAlwaysFails {
								cHealthy.Store(true) // Recover for retry attempt
							}
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

		cfg := createDefaultConfig().(*Config)
		cfg.ClientConfig = clientConfig
		cfg.TopicFromAttribute = "kafka.topic"
		cfg.Producer.MaxMessageBytes = 1024 * 1024
		cfg.TimeoutSettings.Timeout = 500 * time.Millisecond
		cfg.QueueBatchConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
		cfg.BackOffConfig.Enabled = opts.retryEnabled
		cfg.BackOffConfig.InitialInterval = 10 * time.Millisecond
		cfg.BackOffConfig.MaxInterval = 20 * time.Millisecond
		cfg.BackOffConfig.MaxElapsedTime = 60 * time.Millisecond

		set := exportertest.NewNopSettings(metadata.Type)
		exp := newTracesExporter(*cfg, set)

		host := componenttest.NewNopHost()
		kgoClientOpts := []kgo.Opt{
			kgo.SeedBrokers(clientConfig.Brokers...),
			kgo.ClientID(cfg.ClientConfig.ClientID),
			kgo.RecordRetries(0), // return retryable error directly without internal loop
			kgo.WithHooks(kafkaclient.NewStatusReporter(host)),
		}

		client, err := kafka.NewFranzSyncProducer(t.Context(), host, clientConfig,
			cfg.Producer, 500*time.Millisecond, zap.NewNop(), kgoClientOpts...)
		require.NoError(t, err)

		messenger, err := exp.newMessenger(host)
		require.NoError(t, err)
		exp.messenger = messenger
		exp.producer = kafkaclient.NewFranzSyncProducer(
			client,
			exp.cfg.IncludeMetadataKeys,
			exp.cfg.RecordHeaders,
			exp.cfg.Producer.MaxMessageBytes,
			nil,
		)

		var tracesExp exporter.Traces
		if opts.gateEnabled {
			tracesExp, err = xexporterhelper.NewTracesRequest(
				t.Context(),
				set,
				newRequestConverter(exp),
				newRequestPusher(exp),
				exporterhelperOptions(
					*cfg,
					xexporterhelper.QueueBatchSettings{},
					nil, exp.Close,
				)...,
			)
		} else {
			tracesExp, err = exporterhelper.NewTraces(
				t.Context(),
				set,
				cfg,
				exp.exportData,
				exporterhelperOptions(
					*cfg,
					xexporterhelper.QueueBatchSettings{},
					nil, exp.Close,
				)...,
			)
		}
		require.NoError(t, err)
		require.NoError(t, tracesExp.Start(t.Context(), host))
		t.Cleanup(func() {
			_ = tracesExp.Shutdown(context.Background())
		})

		return tracesExp, &topicASends, &topicBSends, &topicCSends
	}

	t.Run("gate_enabled", func(t *testing.T) {
		exp, topicASends, topicBSends, topicCSends := setupExporter(t, exporterTestOpts{
			gateEnabled:  true,
			retryEnabled: true,
		})

		err := exp.ConsumeTraces(t.Context(), createTraces())
		require.NoError(t, err)

		assert.Equal(t, int32(1), topicASends.Load(), "topicA must not be resent on retry")
		assert.Equal(t, int32(1), topicBSends.Load(), "topicB must not be resent on retry")
		assert.Equal(t, int32(2), topicCSends.Load(), "topicC must be attempted on attempt 1 and retried on attempt 2")
	})

	t.Run("gate_disabled", func(t *testing.T) {
		exp, topicASends, topicBSends, topicCSends := setupExporter(t, exporterTestOpts{
			gateEnabled:  false,
			retryEnabled: true,
		})

		err := exp.ConsumeTraces(t.Context(), createTraces())
		require.Error(t, err)
		assert.True(t, consumererror.IsPermanent(err), "pdata path with permanent failure must be classified permanent")

		assert.Equal(t, int32(1), topicASends.Load(), "topicA must not be resent on pdata path")
		assert.Equal(t, int32(1), topicBSends.Load(), "topicB must not be resent on pdata path")
		assert.Equal(t, int32(1), topicCSends.Load(), "topicC must not be retried when mixed batch is marked permanent")
	})

	t.Run("gate_enabled_retry_exhaustion", func(t *testing.T) {
		exp, topicASends, topicBSends, topicCSends := setupExporter(t, exporterTestOpts{
			gateEnabled:  true,
			retryEnabled: true,
			cAlwaysFails: true,
		})

		err := exp.ConsumeTraces(t.Context(), createTraces())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no more retries left")

		assert.Equal(t, int32(1), topicASends.Load(), "topicA must not be resent on retry")
		assert.Equal(t, int32(1), topicBSends.Load(), "topicB must not be resent on retry")
		assert.Greater(t, topicCSends.Load(), int32(1), "topicC must be retried until retries exhaust")
	})

	t.Run("gate_enabled_retry_disabled", func(t *testing.T) {
		exp, topicASends, topicBSends, topicCSends := setupExporter(t, exporterTestOpts{
			gateEnabled:  true,
			retryEnabled: false,
		})

		err := exp.ConsumeTraces(t.Context(), createTraces())
		require.Error(t, err)

		assert.Equal(t, int32(1), topicASends.Load(), "topicA sent once")
		assert.Equal(t, int32(1), topicBSends.Load(), "topicB sent once")
		assert.Equal(t, int32(1), topicCSends.Load(), "topicC must not be retried when retry_on_failure is disabled")
	})
}
