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

package agent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestStartAgentSuccess(t *testing.T) {
	logger := zap.NewNop().Sugar()
	pipeline := &testutil.Pipeline{}
	persister := testutil.NewMockPersister("test")
	pipeline.On("Start", persister).Return(nil)

	agent := LogAgent{
		SugaredLogger: logger,
		pipeline:      pipeline,
	}
	err := agent.Start(persister)
	require.NoError(t, err)
	pipeline.AssertCalled(t, "Start", persister)
}

func TestStartAgentFailure(t *testing.T) {
	logger := zap.NewNop().Sugar()
	pipeline := &testutil.Pipeline{}
	persister := testutil.NewMockPersister("test")
	failure := fmt.Errorf("failed to start pipeline")
	pipeline.On("Start", persister).Return(failure)

	agent := LogAgent{
		SugaredLogger: logger,
		pipeline:      pipeline,
	}
	err := agent.Start(persister)
	require.Error(t, err, failure)
	pipeline.AssertCalled(t, "Start", persister)
}

func TestStopAgentSuccess(t *testing.T) {
	logger := zap.NewNop().Sugar()
	pipeline := &testutil.Pipeline{}
	pipeline.On("Stop").Return(nil)

	agent := LogAgent{
		SugaredLogger: logger,
		pipeline:      pipeline,
	}
	err := agent.Stop()
	require.NoError(t, err)
	pipeline.AssertCalled(t, "Stop")
}

func TestStopAgentPipelineFailure(t *testing.T) {
	logger := zap.NewNop().Sugar()
	pipeline := &testutil.Pipeline{}
	failure := fmt.Errorf("failed to start pipeline")
	pipeline.On("Stop").Return(failure)

	agent := LogAgent{
		SugaredLogger: logger,
		pipeline:      pipeline,
	}
	err := agent.Stop()
	require.Error(t, err, failure)
	pipeline.AssertCalled(t, "Stop")
}
