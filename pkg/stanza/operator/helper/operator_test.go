// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"

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

	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.NoError(t, err)
}

func TestBasicConfigBuildWithoutType(t *testing.T) {
	config := BasicConfig{
		OperatorID: "test-id",
	}

	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "missing required `type` field.")
}

func TestBasicConfigBuildMissingLogger(t *testing.T) {
	config := BasicConfig{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = nil
	_, err := config.Build(set)
	require.ErrorContains(t, err, "operator build context is missing a logger.")
}

func TestBasicConfigBuildValid(t *testing.T) {
	config := BasicConfig{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}

	set := componenttest.NewNopTelemetrySettings()
	operator, err := config.Build(set)
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
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zaptest.NewLogger(t)
	operator := BasicOperator{
		OperatorID:   "test-id",
		OperatorType: "test-type",
		set:          set,
	}
	require.Equal(t, set.Logger, operator.Logger())
}

func TestBasicOperatorStart(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test-id",
		OperatorType: "test-type",
	}
	err := operator.Start(testutil.NewUnscopedMockPersister())
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
