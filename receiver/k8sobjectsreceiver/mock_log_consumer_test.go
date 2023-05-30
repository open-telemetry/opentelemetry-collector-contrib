// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobjectsreceiver

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

type mockLogConsumer struct {
	logs  []plog.Logs
	count int
	lock  sync.Mutex
}

func newMockLogConsumer() *mockLogConsumer {
	return &mockLogConsumer{
		logs: make([]plog.Logs, 0),
		lock: sync.Mutex{},
	}
}

func (m *mockLogConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m *mockLogConsumer) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.logs = append(m.logs, ld)
	m.count += ld.LogRecordCount()
	return nil
}

func (m *mockLogConsumer) Count() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.count
}

func (m *mockLogConsumer) Logs() []plog.Logs {
	m.lock.Lock()
	defer m.lock.Unlock()
	logs := make([]plog.Logs, len(m.logs))
	for i, log := range m.logs {
		l := plog.NewLogs()
		log.CopyTo(l)
		logs[i] = l
	}

	return logs
}
