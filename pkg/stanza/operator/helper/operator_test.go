// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBasicConfigBuildValid(t *testing.T) {
	config := BasicConfig{
		operator.Identity{
			ID:   "test-id",
			Type: "test-type",
		},
	}

	operator, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.Equal(t, "test-id", operator.OperatorID)
	require.Equal(t, "test-type", operator.OperatorType)
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
