// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consumerretry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"

import (
	"context"
	"errors"
	"sync/atomic"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

type MockLogsRejecter struct {
	consumertest.LogsSink
	rejectCount *atomic.Int32
	acceptAfter int32
}

// NewMockLogsRejecter creates new MockLogsRejecter. acceptAfter is a number of rejects before accepting,
// 0 means always accept, -1 means always reject with permanent error
func NewMockLogsRejecter(acceptAfter int32) *MockLogsRejecter {
	return &MockLogsRejecter{
		acceptAfter: acceptAfter,
		rejectCount: &atomic.Int32{},
	}
}

func (m *MockLogsRejecter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	if m.acceptAfter < 0 {
		return consumererror.NewPermanent(errors.New("permanent error"))
	}
	if m.rejectCount.Load() < m.acceptAfter {
		m.rejectCount.Add(1)
		return errors.New("retry later")
	}
	return m.LogsSink.ConsumeLogs(ctx, logs)
}

// mockPartialLogsRejecter is a mock LogsConsumer that accepts only one logs object and rejects the rest.
type mockPartialLogsRejecter struct {
	consumertest.LogsSink
}

func (m *mockPartialLogsRejecter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	if logs.ResourceLogs().Len() <= 1 {
		return m.LogsSink.ConsumeLogs(ctx, logs)
	}
	accepted := plog.NewLogs()
	rejected := plog.NewLogs()
	logs.ResourceLogs().At(0).CopyTo(accepted.ResourceLogs().AppendEmpty())
	for i := 1; i < logs.ResourceLogs().Len(); i++ {
		logs.ResourceLogs().At(i).CopyTo(rejected.ResourceLogs().AppendEmpty())
	}
	_ = m.LogsSink.ConsumeLogs(ctx, accepted)
	return consumererror.NewLogs(errors.New("partial error"), rejected)
}
