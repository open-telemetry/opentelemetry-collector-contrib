// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func (m *mockLogConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
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
		copy := plog.NewLogs()
		log.CopyTo(copy)
		logs[i] = copy
	}

	return logs
}
