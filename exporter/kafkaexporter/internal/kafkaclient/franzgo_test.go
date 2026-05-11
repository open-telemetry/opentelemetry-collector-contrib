// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
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

	producer := NewFranzSyncProducer(client, nil, nil, maxMessageBytes, nil)

	// Create a message larger than maxMessageBytes to trigger MessageTooLarge.
	largeValue := []byte(strings.Repeat("x", maxMessageBytes*2))
	msgs := []Message{{Topic: topic, Value: largeValue}}

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
	recordHeaders := []RecordHeader{
		{Name: "static-key-ONLY", Value: configopaque.String("static-value")},
		{Name: "shared-key", Value: configopaque.String("static-value-override")},
	}

	md := client.NewMetadata(map[string][]string{
		"dynamic-key-ONLY": {"dynamic-value"},
		"shared-key":       {"dynamic-value-wins"},
	})
	ctx := client.NewContext(t.Context(), client.Info{Metadata: md})

	msgs := []Message{{Topic: "test-topic", Value: []byte("test-payload")}}

	// NewFranzSyncProducer will convert recordHeaders to kgo.RecordHeader and store them in the producer struct.
	producer := NewFranzSyncProducer(nil, nil, recordHeaders, 0, nil)
	metadataHeaders := metadataToHeaders(ctx, []string{"dynamic-key-ONLY", "shared-key"})

	records := makeFranzMessages(msgs, producer.recordHeaders, metadataHeaders)

	require.Len(t, records, 1, "expected exactly 1 record")
	record := records[0]

	assert.Equal(t, "test-topic", record.Topic)
	assert.Equal(t, []byte("test-payload"), record.Value)

	require.Len(t, record.Headers, 4, "expected exactly 4 headers on the record")

	headerMap := make(map[string]string)
	for _, h := range record.Headers {
		headerMap[h.Key] = string(h.Value)
	}

	assert.Equal(t, "static-value", headerMap["static-key-ONLY"], "static headers unique key failed")
	assert.Equal(t, "dynamic-value", headerMap["dynamic-key-ONLY"], "dynamic headers unique key failed")
	assert.Equal(t, "dynamic-value-wins", headerMap["shared-key"], "Precedence for common key failed")
}

func TestClose_UnblocksInFlightExportData(t *testing.T) {
	fakeCluster, err := kfake.NewCluster(kfake.NumBrokers(1))
	require.NoError(t, err)

	clientCtx, clientCancel := context.WithCancel(t.Context())
	kgoClient, err := kgo.NewClient(
		kgo.SeedBrokers(fakeCluster.ListenAddrs()[0]),
		kgo.WithContext(clientCtx),
	)
	require.NoError(t, err)
	t.Cleanup(kgoClient.Close)

	// Shut down the broker so ExportData blocks indefinitely.
	fakeCluster.Close()

	producer := NewFranzSyncProducer(kgoClient, nil, nil, 1024*1024, clientCancel)

	msgs := []Message{{Topic: "otlp_logs", Value: []byte("test")}}

	exportDone := make(chan error, 1)
	go func() { exportDone <- producer.ExportData(t.Context(), msgs) }()

	// Close must return and unblock ExportData within the deadline.
	closeCtx, closeCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer closeCancel()

	err = producer.Close(closeCtx)
	if err != nil {
		require.ErrorIs(t, err, context.DeadlineExceeded, "Close returned unexpected error")
	}

	select {
	case <-exportDone:
	case <-closeCtx.Done():
		t.Fatal("ExportData was not unblocked by Close; collector would hang on shutdown")
	}
}
