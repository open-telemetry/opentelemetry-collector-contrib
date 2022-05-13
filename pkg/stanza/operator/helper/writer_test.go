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

package helper

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestWriterConfigMissingOutput(t *testing.T) {
	config := WriterConfig{
		BasicConfig: BasicConfig{
			OperatorType: "testtype",
		},
	}
	_, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
}

func TestWriterConfigValidBuild(t *testing.T) {
	config := WriterConfig{
		OutputIDs: OutputIDs{"output"},
		BasicConfig: BasicConfig{
			OperatorType: "testtype",
		},
	}
	_, err := config.Build(testutil.Logger(t))
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
		OutputIDs: OutputIDs{"output2"},
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
		OutputIDs: OutputIDs{"output1"},
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
		OutputIDs: OutputIDs{"output1", "output2"},
	}

	err := writer.SetOutputs([]operator.Operator{output1, output2})
	require.NoError(t, err)
	require.Equal(t, []operator.Operator{output1, output2}, writer.Outputs())
}

func TestUnmarshalJSONString(t *testing.T) {
	bytes := []byte("{\"output\":\"test\"}")
	var config WriterConfig
	err := json.Unmarshal(bytes, &config)
	require.NoError(t, err)
	require.Equal(t, OutputIDs{"test"}, config.OutputIDs)
}

func TestUnmarshalJSONArray(t *testing.T) {
	bytes := []byte("{\"output\":[\"test1\",\"test2\"]}")
	var config WriterConfig
	err := json.Unmarshal(bytes, &config)
	require.NoError(t, err)
	require.Equal(t, OutputIDs{"test1", "test2"}, config.OutputIDs)
}

func TestUnmarshalJSONInvalidValue(t *testing.T) {
	bytes := []byte("..")
	var config WriterConfig
	err := json.Unmarshal(bytes, &config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid character")
}

func TestUnmarshalJSONInvalidString(t *testing.T) {
	bytes := []byte("{\"output\": true}")
	var config WriterConfig
	err := json.Unmarshal(bytes, &config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value is not of type string or string array")
}

func TestUnmarshalJSONInvalidArray(t *testing.T) {
	bytes := []byte("{\"output\":[\"test1\", true]}")
	var config WriterConfig
	err := json.Unmarshal(bytes, &config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value in array is not of type string")
}

func TestUnmarshalYAMLString(t *testing.T) {
	bytes := []byte("output: test")
	var config WriterConfig
	err := yaml.Unmarshal(bytes, &config)
	require.NoError(t, err)
	require.Equal(t, OutputIDs{"test"}, config.OutputIDs)
}

func TestUnmarshalYAMLArray(t *testing.T) {
	bytes := []byte("output: [test1, test2]")
	var config WriterConfig
	err := yaml.Unmarshal(bytes, &config)
	require.NoError(t, err)
	require.Equal(t, OutputIDs{"test1", "test2"}, config.OutputIDs)
}

func TestUnmarshalYAMLInvalidValue(t *testing.T) {
	bytes := []byte("..")
	var config WriterConfig
	err := yaml.Unmarshal(bytes, &config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot unmarshal")
}

func TestUnmarshalYAMLInvalidString(t *testing.T) {
	bytes := []byte("output: true")
	var config WriterConfig
	err := yaml.Unmarshal(bytes, &config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value is not of type string or string array")
}

func TestUnmarshalYAMLInvalidArray(t *testing.T) {
	bytes := []byte("output: [test1, true]")
	var config WriterConfig
	err := yaml.Unmarshal(bytes, &config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value in array is not of type string")
}
