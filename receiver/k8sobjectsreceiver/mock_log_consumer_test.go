package k8sobjectreceiver

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

type mockLogConsumer struct {
	Logs  []plog.Logs
	Count int
}

func newMockLogConsumer() *mockLogConsumer {
	return &mockLogConsumer{
		Logs: make([]plog.Logs, 0),
	}
}

func (m *mockLogConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m *mockLogConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	m.Logs = append(m.Logs, ld)
	m.Count += ld.LogRecordCount()
	return nil
}
