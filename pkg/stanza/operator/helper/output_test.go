// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestOutputConfigMissingBase(t *testing.T) {
	config := OutputConfig{}
	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required `type` field.")
}

func TestOutputConfigBuildValid(t *testing.T) {
	config := OutputConfig{
		BasicConfig: BasicConfig{
			OperatorID:   "test-id",
			OperatorType: "test-type",
		},
	}
	_, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestOutputOperatorCanProcess(t *testing.T) {
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:    "test-id",
			OperatorType:  "test-type",
			SugaredLogger: testutil.Logger(t),
		},
	}
	require.True(t, output.CanProcess())
}

func TestOutputOperatorCanOutput(t *testing.T) {
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:    "test-id",
			OperatorType:  "test-type",
			SugaredLogger: testutil.Logger(t),
		},
	}
	require.False(t, output.CanOutput())
}

func TestOutputOperatorOutputs(t *testing.T) {
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:    "test-id",
			OperatorType:  "test-type",
			SugaredLogger: testutil.Logger(t),
		},
	}
	require.Equal(t, []operator.Operator{}, output.Outputs())
}

func TestOutputOperatorSetOutputs(t *testing.T) {
	output := OutputOperator{
		BasicOperator: BasicOperator{
			OperatorID:    "test-id",
			OperatorType:  "test-type",
			SugaredLogger: testutil.Logger(t),
		},
	}

	err := output.SetOutputs([]operator.Operator{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Operator can not output")
}
