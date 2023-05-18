// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBasicConfigID(t *testing.T) {
	config := BasicConfig{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	require.Equal(t, "test-id", config.ID())
}

func TestBasicConfigType(t *testing.T) {
	config := BasicConfig{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	require.Equal(t, "test-type", config.Type())
}

func TestBasicConfigBuildWithoutID(t *testing.T) {
	config := BasicConfig{
		OperatorType: "test-type",
	}

	_, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestBasicConfigBuildWithoutType(t *testing.T) {
	config := BasicConfig{
		OperatorID: "test-id",
	}

	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required `type` field.")
}

func TestBasicConfigBuildMissingLogger(t *testing.T) {
	config := BasicConfig{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}

	_, err := config.Build(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "operator build context is missing a logger.")
}

func TestBasicConfigBuildValid(t *testing.T) {
	config := BasicConfig{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}

	operator, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.Equal(t, "test-id", operator.OperatorID)
	require.Equal(t, "test-type", operator.OperatorType)
}

func TestBasicOperatorID(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	require.Equal(t, "test-id", operator.ID())
}

func TestBasicOperatorType(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	require.Equal(t, "test-type", operator.Type())
}

func TestBasicOperatorLogger(t *testing.T) {
	logger := &zap.SugaredLogger{}
	operator := BasicOperator{
		OperatorID:    "test-id",
		OperatorType:  "test-type",
		SugaredLogger: logger,
	}
	require.Equal(t, logger, operator.Logger())
}

func TestBasicOperatorStart(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	err := operator.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)
}

func TestBasicOperatorStop(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	err := operator.Stop()
	require.NoError(t, err)
}
