// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestInputConfigMissingBase(t *testing.T) {
	config := InputConfig{
		WriterConfig: WriterConfig{
			OutputIDs: []string{"test-output"},
		},
	}

	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required `type` field.")
}

func TestInputConfigMissingOutput(t *testing.T) {
	config := InputConfig{
		WriterConfig: WriterConfig{
			BasicConfig: BasicConfig{
				OperatorID:   "test-id",
				OperatorType: "test-type",
			},
		},
	}

	_, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestInputConfigValid(t *testing.T) {
	config := InputConfig{
		WriterConfig: WriterConfig{
			BasicConfig: BasicConfig{
				OperatorID:   "test-id",
				OperatorType: "test-type",
			},
			OutputIDs: []string{"test-output"},
		},
	}

	_, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestInputOperatorCanProcess(t *testing.T) {
	input := InputOperator{
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:    "test-id",
				OperatorType:  "test-type",
				SugaredLogger: testutil.Logger(t),
			},
		},
	}
	require.False(t, input.CanProcess())
}

func TestInputOperatorProcess(t *testing.T) {
	input := InputOperator{
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:    "test-id",
				OperatorType:  "test-type",
				SugaredLogger: testutil.Logger(t),
			},
		},
	}
	entry := entry.New()
	ctx := context.Background()
	err := input.Process(ctx, entry)
	require.Error(t, err)
	require.Equal(t, err.Error(), "Operator can not process logs.")
}

func TestInputOperatorNewEntry(t *testing.T) {
	body := entry.NewBodyField()

	labelExpr, err := ExprStringConfig("test").Build()
	require.NoError(t, err)

	resourceExpr, err := ExprStringConfig("resource").Build()
	require.NoError(t, err)

	input := InputOperator{
		Attributer: Attributer{
			attributes: map[string]*ExprString{
				"test-label": labelExpr,
			},
		},
		Identifier: Identifier{
			resource: map[string]*ExprString{
				"resource-key": resourceExpr,
			},
		},
		WriterOperator: WriterOperator{
			BasicOperator: BasicOperator{
				OperatorID:    "test-id",
				OperatorType:  "test-type",
				SugaredLogger: testutil.Logger(t),
			},
		},
	}

	entry, err := input.NewEntry("test")
	require.NoError(t, err)

	value, exists := entry.Get(body)
	require.True(t, exists)
	require.Equal(t, "test", value)

	labelValue, exists := entry.Attributes["test-label"]
	require.True(t, exists)
	require.Equal(t, "test", labelValue)

	resourceValue, exists := entry.Resource["resource-key"]
	require.True(t, exists)
	require.Equal(t, "resource", resourceValue)
}
