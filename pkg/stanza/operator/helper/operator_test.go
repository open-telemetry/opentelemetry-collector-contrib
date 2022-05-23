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
