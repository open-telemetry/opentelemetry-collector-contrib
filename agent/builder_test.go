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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/builtin/transformer/noop"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestBuildAgentSuccess(t *testing.T) {
	mockCfg := Config{
		[]operator.Config{
			{
				Builder: noop.NewNoopOperatorConfig("noop"),
			},
		},
	}
	mockLogger := zap.NewNop().Sugar()

	agent, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		Build()
	require.NoError(t, err)
	require.Equal(t, mockLogger, agent.SugaredLogger)
}

func TestBuildAgentDefaultOperator(t *testing.T) {
	mockCfg := Config{
		[]operator.Config{
			{
				Builder: noop.NewNoopOperatorConfig("noop"),
			},
			{
				Builder: noop.NewNoopOperatorConfig("noop1"),
			},
		},
	}
	mockLogger := zap.NewNop().Sugar()
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithConfig(&mockCfg).
		WithDefaultOutput(mockOutput).
		Build()
	require.NoError(t, err)
	require.Equal(t, mockLogger, agent.SugaredLogger)

	ops := agent.pipeline.Operators()
	require.Equal(t, 3, len(ops))

	exists := make(map[string]bool)

	for _, op := range ops {
		switch op.ID() {
		case "$.noop":
			require.Equal(t, 1, len(op.GetOutputIDs()))
			require.Equal(t, "$.noop1", op.GetOutputIDs()[0])
			exists["$.noop"] = true
		case "$.noop1":
			require.Equal(t, 1, len(op.GetOutputIDs()))
			require.Equal(t, "$.fake", op.GetOutputIDs()[0])
			exists["$.noop1"] = true
		case "$.fake":
			require.Equal(t, 0, len(op.GetOutputIDs()))
			exists["$.fake"] = true
		}
	}
	require.True(t, exists["$.noop"])
	require.True(t, exists["$.noop1"])
	require.True(t, exists["$.fake"])
}

func TestBuildAgentFailureNoConfigOrGlobs(t *testing.T) {
	mockLogger := zap.NewNop().Sugar()
	mockOutput := testutil.NewFakeOutput(t)

	agent, err := NewBuilder(mockLogger).
		WithDefaultOutput(mockOutput).
		Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "config must be specified")
	require.Nil(t, agent)
}
