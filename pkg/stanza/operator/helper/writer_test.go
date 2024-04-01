// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestWriterConfigMissingOutput(t *testing.T) {
	c := WriterConfig{
		BasicConfig: BasicConfig{
			Identity: operator.Identity{
				Type: "test_type",
				Name: "test-id",
			},
		},
	}
	_, err := NewWriter(componenttest.NewNopTelemetrySettings(), c)
	require.NoError(t, err)
}

func TestWriterConfigValidBuild(t *testing.T) {
	c := WriterConfig{
		OutputIDs: []string{"output"},
		BasicConfig: BasicConfig{
			Identity: operator.Identity{
				Type: "test_type",
				Name: "test-id",
			},
		},
	}
	_, err := NewWriter(componenttest.NewNopTelemetrySettings(), c)
	require.NoError(t, err)
}

func TestWriterOperatorWrite(t *testing.T) {
	output1 := &testutil.Operator{}
	output1.On("Process", mock.Anything, mock.Anything).Return(nil)
	output2 := &testutil.Operator{}
	output2.On("Process", mock.Anything, mock.Anything).Return(nil)
	writer := WriterOperator{
		OutputOperators: []operator.Operator{output1, output2},
	}

	ctx := context.Background()
	testEntry := entry.New()

	writer.Write(ctx, testEntry)
	output1.AssertCalled(t, "Process", ctx, mock.Anything)
	output2.AssertCalled(t, "Process", ctx, mock.Anything)
}

func TestWriterOperatorCanOutput(t *testing.T) {
	writer := WriterOperator{}
	require.True(t, writer.CanOutput())
}

func TestWriterOperatorOutputs(t *testing.T) {
	output1 := &testutil.Operator{}
	output1.On("Process", mock.Anything, mock.Anything).Return(nil)
	output2 := &testutil.Operator{}
	output2.On("Process", mock.Anything, mock.Anything).Return(nil)
	writer := WriterOperator{
		OutputOperators: []operator.Operator{output1, output2},
	}

	ctx := context.Background()
	testEntry := entry.New()

	writer.Write(ctx, testEntry)
	output1.AssertCalled(t, "Process", ctx, mock.Anything)
	output2.AssertCalled(t, "Process", ctx, mock.Anything)
}

func TestWriterSetOutputsMissing(t *testing.T) {
	output1 := &testutil.Operator{}
	output1.On("ID").Return("output1")
	writer := WriterOperator{
		OutputIDs: []string{"output2"},
	}

	err := writer.SetOutputs([]operator.Operator{output1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist")
}

func TestWriterSetOutputsInvalid(t *testing.T) {
	output1 := &testutil.Operator{}
	output1.On("ID").Return("output1")
	output1.On("CanProcess").Return(false)
	writer := WriterOperator{
		OutputIDs: []string{"output1"},
	}

	err := writer.SetOutputs([]operator.Operator{output1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "can not process entries")
}

func TestWriterSetOutputsValid(t *testing.T) {
	output1 := &testutil.Operator{}
	output1.On("ID").Return("output1")
	output1.On("CanProcess").Return(true)
	output2 := &testutil.Operator{}
	output2.On("ID").Return("output2")
	output2.On("CanProcess").Return(true)
	writer := WriterOperator{
		OutputIDs: []string{"output1", "output2"},
	}

	err := writer.SetOutputs([]operator.Operator{output1, output2})
	require.NoError(t, err)
	require.Equal(t, []operator.Operator{output1, output2}, writer.Outputs())
}

func TestUnmarshalWriterConfig(t *testing.T) {
	f := newHelpersFactory()
	operatortest.ConfigUnmarshalTests{
		Factory:   f,
		TestsFile: filepath.Join(".", "testdata", "writer.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "string",
				Expect: func() *helpersConfig {
					c := f.NewDefaultConfig("").(*helpersConfig)
					c.WriterConfig.OutputIDs = []string{"test"}
					return c
				}(),
			},
			{
				Name: "slice",
				Expect: func() *helpersConfig {
					c := f.NewDefaultConfig("").(*helpersConfig)
					c.WriterConfig.OutputIDs = []string{"test1", "test2"}
					return c
				}(),
			},
			{
				Name:      "invalid",
				ExpectErr: true,
			},
		},
	}.Run(t)
}
