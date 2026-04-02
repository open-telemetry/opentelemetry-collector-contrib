// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

func TestExportData_MessageTooLarge(t *testing.T) {
	const (
		topic           = "test-topic"
		maxMessageBytes = 512
	)
	cluster, err := kfake.NewCluster(kfake.SeedTopics(1, topic))
	require.NoError(t, err)
	t.Cleanup(cluster.Close)

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cluster.ListenAddrs()...),
		kgo.ProducerBatchMaxBytes(int32(maxMessageBytes)),
	)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	producer := NewFranzSyncProducer(client, nil, nil, maxMessageBytes)

	// Create a message larger than maxMessageBytes to trigger MessageTooLarge.
	largeValue := []byte(strings.Repeat("x", maxMessageBytes*2))
	msgs := Messages{
		Count: 1,
		TopicMessages: []TopicMessages{{
			Topic: topic,
			Messages: []marshaler.Message{{
				Value: largeValue,
			}},
		}},
	}

	err = producer.ExportData(t.Context(), msgs)
	require.Error(t, err)

	// Verify the error is permanent and wraps MessageTooLarge.
	assert.True(t, consumererror.IsPermanent(err), "expected permanent error")
	require.ErrorIs(t, err, kerr.MessageTooLarge, "expected MessageTooLarge error")

	// Verify the error wraps MessageTooLargeError with correct sizes.
	var msgTooLarge *MessageTooLargeError
	require.ErrorAs(t, err, &msgTooLarge)
	assert.Equal(t, len(largeValue), msgTooLarge.RecordBytes)
	assert.Equal(t, maxMessageBytes, msgTooLarge.MaxMessageBytes)

	// Verify sizes appear in the error string for pipeline-level visibility.
	assert.Contains(t, err.Error(), "record size")
	assert.Contains(t, err.Error(), "exceeds max")
}

func TestMakeFranzMessages_RecordHeaders(t *testing.T) {
	recordHeaders := configopaque.MapList{
		{Name: "custom-header-1", Value: configopaque.String("value-1")},
		{Name: "custom-header-2", Value: configopaque.String("value-2")},
	}

	md := client.NewMetadata(map[string][]string{
		"dynamic-key": {"dynamic-value"},
	})
	ctx := client.NewContext(t.Context(), client.Info{Metadata: md})

	msgs := Messages{
		Count: 1,
		TopicMessages: []TopicMessages{{
			Topic: "test-topic",
			Messages: []marshaler.Message{{
				Value: []byte("test-payload"),
			}},
		}},
	}

	records := makeFranzMessages(msgs, recordHeaders)
	setMessageHeaders(ctx, records, []string{"dynamic-key"})

	require.Len(t, records, 1, "expected exactly 1 record")
	record := records[0]

	assert.Equal(t, "test-topic", record.Topic)
	assert.Equal(t, []byte("test-payload"), record.Value)

	require.Len(t, record.Headers, 3, "expected exactly 3 headers (2 static, 1 dynamic)")

	headerMap := make(map[string]string)
	for _, h := range record.Headers {
		headerMap[h.Key] = string(h.Value)
	}

	assert.Equal(t, "value-1", headerMap["custom-header-1"])
	assert.Equal(t, "value-2", headerMap["custom-header-2"])
	assert.Equal(t, "dynamic-value", headerMap["dynamic-key"])
}
