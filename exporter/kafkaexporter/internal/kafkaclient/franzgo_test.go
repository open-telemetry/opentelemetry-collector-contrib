// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

var (
	_ component.Host           = (*statusRecorderHost)(nil)
	_ componentstatus.Reporter = (*statusRecorderHost)(nil)
)

// statusRecorderHost implements component.Host and componentstatus.Reporter so
// componentstatus.ReportStatus records events for tests.
type statusRecorderHost struct {
	mu     sync.Mutex
	events []*componentstatus.Event
}

func (h *statusRecorderHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (h *statusRecorderHost) Report(event *componentstatus.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
}

func (h *statusRecorderHost) snapshot() []*componentstatus.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*componentstatus.Event(nil), h.events...)
}

type statusStepKind int

const (
	statusStepConnect statusStepKind = iota
	statusStepDisconnect
)

// statusStep is one broker hook invocation used in table-driven tests.
type statusStep struct {
	kind statusStepKind
	meta kgo.BrokerMetadata
	err  error // connect outcome; ignored for disconnect
}

func (st statusStep) apply(r *statusReporter) {
	switch st.kind {
	case statusStepConnect:
		r.OnBrokerConnect(st.meta, 0, nil, st.err)
	case statusStepDisconnect:
		r.OnBrokerDisconnect(st.meta, nil)
	}
}

type wantStatusEvent struct {
	status componentstatus.Status
	errIs  error // if nil, assert no error on the event (e.g. StatusOK)
}

func assertStatusEvents(t *testing.T, got []*componentstatus.Event, want []wantStatusEvent) {
	t.Helper()
	require.Len(t, got, len(want), "event count")
	for i := range want {
		assert.Equal(t, want[i].status, got[i].Status(), "event[%d] status", i)
		if want[i].errIs != nil {
			assert.ErrorIs(t, got[i].Err(), want[i].errIs, "event[%d] err", i)
		} else {
			assert.NoError(t, got[i].Err(), "event[%d] err", i)
		}
	}
}

// TestStatusReporter_ComponentStatus exercises broker hook → componentstatus
// reporting (table-driven: each row is an isolated scenario: arrange steps,
// act via apply, assert on recorded events).
func TestStatusReporter_ComponentStatus(t *testing.T) {
	broker1 := kgo.BrokerMetadata{NodeID: 1, Host: "127.0.0.1", Port: 9092}
	broker2 := kgo.BrokerMetadata{NodeID: 2, Host: "127.0.0.2", Port: 9092}
	errRefused := errors.New("connection refused")
	errBroker2 := errors.New("broker 2 down")
	errUnreachable := errors.New("cluster unreachable")

	tests := []struct {
		name   string
		steps  []statusStep
		wantEv []wantStatusEvent
	}{
		{
			name:   "successful_connect_reports_OK",
			steps:  []statusStep{{kind: statusStepConnect, meta: broker1, err: nil}},
			wantEv: []wantStatusEvent{{status: componentstatus.StatusOK}},
		},
		{
			name:   "failed_connect_with_no_open_broker_reports_recoverable",
			steps:  []statusStep{{kind: statusStepConnect, meta: broker1, err: errRefused}},
			wantEv: []wantStatusEvent{{status: componentstatus.StatusRecoverableError, errIs: errRefused}},
		},
		{
			name: "failed_connect_suppressed_when_another_broker_connected",
			steps: []statusStep{
				{kind: statusStepConnect, meta: broker1, err: nil},
				{kind: statusStepConnect, meta: broker2, err: errBroker2},
			},
			wantEv: []wantStatusEvent{{status: componentstatus.StatusOK}},
		},
		{
			name: "disconnect_then_failed_connect_reports_recoverable",
			steps: []statusStep{
				{kind: statusStepConnect, meta: broker1, err: nil},
				{kind: statusStepDisconnect, meta: broker1},
				{kind: statusStepConnect, meta: broker1, err: errUnreachable},
			},
			wantEv: []wantStatusEvent{
				{status: componentstatus.StatusOK},
				{status: componentstatus.StatusRecoverableError, errIs: errUnreachable},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &statusRecorderHost{}
			reporter := NewStatusReporter(host)

			for _, step := range tt.steps {
				step.apply(reporter)
			}

			assertStatusEvents(t, host.snapshot(), tt.wantEv)
		})
	}
}

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

	producer := NewFranzSyncProducer(client, nil, nil, maxMessageBytes, componenttest.NewNopHost(), nil)

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
	recordHeaders := []RecordHeader{
		{Name: "static-key-ONLY", Value: configopaque.String("static-value")},
		{Name: "shared-key", Value: configopaque.String("static-value-override")},
	}

	md := client.NewMetadata(map[string][]string{
		"dynamic-key-ONLY": {"dynamic-value"},
		"shared-key":       {"dynamic-value-wins"},
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

	// NewFranzSyncProducer will convert recordHeaders to kgo.RecordHeader and store them in the producer struct.
	producer := NewFranzSyncProducer(nil, nil, recordHeaders, 0, nil, nil)
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

	producer := NewFranzSyncProducer(kgoClient, nil, nil, 1024*1024, nil, clientCancel)

	msgs := Messages{
		Count: 1,
		TopicMessages: []TopicMessages{{
			Topic:    "otlp_logs",
			Messages: []marshaler.Message{{Value: []byte("test")}},
		}},
	}

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
