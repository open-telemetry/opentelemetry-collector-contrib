// Copyright  The OpenTelemetry Authors
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
