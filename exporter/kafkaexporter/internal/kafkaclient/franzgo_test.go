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

	producer := NewFranzSyncProducer(client, nil, maxMessageBytes)

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
