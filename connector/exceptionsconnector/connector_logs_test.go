// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exceptionsconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector/mocks"
)

func TestConnectorLogConsumeTraces(t *testing.T) {
	traces := []ptrace.Traces{buildSampleTrace()}
	lcon := &mocks.LogsConsumer{}

	//var wg sync.WaitGroup
	// Mocked log exporter will perform validation on logs, during p.ConsumeLogs()
	lcon.On("ConsumeLogs", mock.Anything, mock.MatchedBy(func(input plog.Logs) bool {
		return verifyConsumeLogsInput(t, input)
	})).Return(nil)

	p, err := newTestLogsConnector(t, lcon, zaptest.NewLogger(t))
	require.NoError(t, err)

	ctx := metadata.NewIncomingContext(context.Background(), nil)
	err = p.Start(ctx, componenttest.NewNopHost())
	defer func() { sdErr := p.Shutdown(ctx); require.NoError(t, sdErr) }()
	require.NoError(t, err)

	for _, traces := range traces {
		err = p.ConsumeTraces(ctx, traces)
		assert.NoError(t, err)
	}
}

func newTestLogsConnector(t *testing.T, lcon consumer.Logs, logger *zap.Logger) (*logsConnector, error) {
	c, err := newLogsConnector(logger)
	c.logsConsumer = lcon
	return c, err
}

func verifyConsumeLogsInput(t testing.TB, input plog.Logs) bool {
	require.Equal(t, 3, input.LogRecordCount(), "Should be 1 for each generated span")

	rl := input.ResourceLogs()

	sl := rl.At(0).ScopeLogs()
	require.Equal(t, 1, sl.Len())

	lrs := sl.At(0).LogRecords()
	require.Equal(t, 1, lrs.Len())

	lr := lrs.At(0)
	assert.Equal(t, plog.SeverityNumberError, lr.SeverityNumber())
	assert.Equal(t, "ERROR", lr.SeverityText())

	sn, _ := lr.Attributes().Get("service.name")
	assert.Equal(t, "service-a", sn.AsString())

	et, _ := lr.Attributes().Get("exception.type")
	assert.Equal(t, "Exception", et.AsString())

	em, _ := lr.Attributes().Get("exception.message")
	assert.Equal(t, "Exception message", em.AsString())

	est, _ := lr.Attributes().Get("exception.stacktrace")
	assert.Equal(t, "Exception stacktrace", est.AsString())
	return true
}
