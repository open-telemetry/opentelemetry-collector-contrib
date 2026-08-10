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
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
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

func (*statusRecorderHost) GetExtensions() map[component.ID]component.Component {
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

func (st statusStep) apply(r *StatusReporter) {
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

	producer := NewFranzSyncProducer(client, nil, nil, maxMessageBytes, nil)

	// Create a message larger than maxMessageBytes to trigger MessageTooLarge.
	largeValue := []byte(strings.Repeat("x", maxMessageBytes*2))
	records := []*kgo.Record{{Topic: topic, Value: largeValue}}

	err = producer.ExportData(t.Context(), records)
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

func TestExportData_AttachesHeaders(t *testing.T) {
	const topic = "test-topic"
	cluster, err := kfake.NewCluster(kfake.SeedTopics(1, topic))
	require.NoError(t, err)
	t.Cleanup(cluster.Close)

	kgoClient, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	require.NoError(t, err)
	t.Cleanup(kgoClient.Close)

	ctx := client.NewContext(t.Context(), client.Info{Metadata: client.NewMetadata(map[string][]string{
		"dynamic-key-ONLY": {"dynamic-value"},
		"shared-key":       {"dynamic-value-wins"},
	})})

	producer := NewFranzSyncProducer(kgoClient,
		[]string{"dynamic-key-ONLY", "shared-key"},
		[]RecordHeader{
			{Name: "static-key-ONLY", Value: configopaque.String("static-value")},
			{Name: "shared-key", Value: configopaque.String("static-value-override")},
		},
		1024*1024,
		nil,
	)

	records := []*kgo.Record{{Topic: topic, Value: []byte("test-payload")}}
	require.NoError(t, producer.ExportData(ctx, records))

	require.Len(t, records[0].Headers, 4)
	got := make(map[string]string, len(records[0].Headers))
	for _, h := range records[0].Headers {
		got[h.Key] = string(h.Value)
	}
	assert.Equal(t, map[string]string{
		"static-key-ONLY":  "static-value",
		"dynamic-key-ONLY": "dynamic-value",
		"shared-key":       "dynamic-value-wins",
	}, got)
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

	records := []*kgo.Record{{Topic: "otlp_logs", Value: []byte("test")}}

	exportDone := make(chan error, 1)
	go func() { exportDone <- producer.ExportData(t.Context(), records) }()

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
