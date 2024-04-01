// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBasicConfigBuildValid(t *testing.T) {
	config := NewBasicConfig("test_id", "test_type")
	operator, err := NewBasicOperator(componenttest.NewNopTelemetrySettings(), config)
	require.NoError(t, err)
	require.Equal(t, "test_type", operator.Identity.Type)
	require.Equal(t, "test_id", operator.Identity.Name)
}

func TestBasicOperatorStart(t *testing.T) {
	config := NewBasicConfig("test_id", "test_type")
	operator, err := NewBasicOperator(componenttest.NewNopTelemetrySettings(), config)
	require.NoError(t, err)
	err = operator.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
}

func TestBasicOperatorStop(t *testing.T) {
	config := NewBasicConfig("test_id", "test_type")
	operator, err := NewBasicOperator(componenttest.NewNopTelemetrySettings(), config)
	require.NoError(t, err)
	err = operator.Stop()
	require.NoError(t, err)
}

func TestBasicConfigBuildValidDeprecated(t *testing.T) {
	config := BasicConfig{
		operator.Identity{
			Name: "test_id",
			Type: "test_type",
		},
	}

	operator, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.Equal(t, "test_id", operator.OperatorID)
	require.Equal(t, "test_type", operator.OperatorType)
}

func TestBasicOperatorStartDeprecated(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test_id",
		OperatorType: "test_type",
	}
	err := operator.Start(testutil.NewUnscopedMockPersister())
	require.NoError(t, err)
}

func TestBasicOperatorStopDeprecated(t *testing.T) {
	operator := BasicOperator{
		OperatorID:   "test_id",
		OperatorType: "test_type",
	}
	err := operator.Stop()
	require.NoError(t, err)
}
