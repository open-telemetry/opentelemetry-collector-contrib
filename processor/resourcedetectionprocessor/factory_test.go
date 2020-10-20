// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resourcedetectionprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NoError(t, configcheck.ValidateConfig(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraceProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewTracesNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetricsProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewMetricsNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	lp, err := factory.CreateLogsProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewLogsNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)
}

func TestInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"not-existing"}

	tp, err := factory.CreateTraceProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewTracesNop())
	assert.Error(t, err)
	assert.Nil(t, tp)

	mp, err := factory.CreateMetricsProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewMetricsNop())
	assert.Error(t, err)
	assert.Nil(t, mp)

	lp, err := factory.CreateLogsProcessor(context.Background(), component.ProcessorCreateParams{}, cfg, consumertest.NewLogsNop())
	assert.Error(t, err)
	assert.Nil(t, lp)
}
