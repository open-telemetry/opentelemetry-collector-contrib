// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func TestOutputConfigMissingBase(t *testing.T) {
	config := OutputConfig{}
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "missing required `type` field.")
}

func TestOutputConfigBuildValid(t *testing.T) {
	config := OutputConfig{
		BasicConfig: BasicConfig{
			OperatorID:   "test-id",
			OperatorType: "test-type",
		},
	}
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.NoError(t, err)
}

func TestOutputOperatorCanProcess(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:   "test-id",
			OperatorType: "test-type",
			set:          set,
		},
	}
	require.True(t, output.CanProcess())
}

func TestOutputOperatorCanOutput(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:   "test-id",
			OperatorType: "test-type",
			set:          set,
		},
	}
	require.False(t, output.CanOutput())
}

func TestOutputOperatorOutputs(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:   "test-id",
			OperatorType: "test-type",
			set:          set,
		},
	}
	require.Equal(t, []operator.Operator{}, output.Outputs())
}

func TestOutputOperatorSetOutputs(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:   "test-id",
			OperatorType: "test-type",
			set:          set,
		},
	}

	err := output.SetOutputs([]operator.Operator{})
	require.ErrorContains(t, err, "Operator cannot output")
}
