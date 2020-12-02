// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbyattrsprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

var (
	config = &Config{
		GroupByKeys: []string{"foo"},
	}

	logger, _ = zap.NewDevelopment()

	params = component.ProcessorCreateParams{
		Logger: logger,
	}
)

func TestDefaultConfiguration(t *testing.T) {
	c := createDefaultConfig().(*Config)
	assert.Empty(t, c.GroupByKeys)
}

func TestCreateTestProcessor(t *testing.T) {
	tp, err := createTraceProcessor(context.Background(), params, config, consumertest.NewTracesNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
	assert.Equal(t, true, tp.GetCapabilities().MutatesConsumedData)

	lp, err := createLogsProcessor(context.Background(), params, config, consumertest.NewLogsNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)
	assert.Equal(t, true, lp.GetCapabilities().MutatesConsumedData)
}

func TestNoKeys(t *testing.T) {
	gbap, err := createGroupByAttrsProcessor(logger, []string{})
	assert.Error(t, err)
	assert.Nil(t, gbap)
}

func TestDuplicateKeys(t *testing.T) {
	gbap, err := createGroupByAttrsProcessor(logger, []string{"foo", "foo", ""})
	assert.NoError(t, err)
	assert.NotNil(t, gbap)
	assert.EqualValues(t, []string{"foo"}, gbap.groupByKeys)
}
