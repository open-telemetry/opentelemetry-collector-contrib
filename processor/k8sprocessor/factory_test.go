// Copyright 2020 OpenTelemetry Authors
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

package k8sprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()

	realClient := kubeClientProvider
	kubeClientProvider = newFakeClient

	cfg := factory.CreateDefaultConfig()
	params := component.ProcessorCreateParams{Logger: zap.NewNop()}

	tp, err := factory.CreateTraceProcessor(context.Background(), params, cfg, consumertest.NewTracesNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	mp, err := factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	lp, err := factory.CreateLogsProcessor(context.Background(), params, cfg, consumertest.NewLogsNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)

	oCfg := cfg.(*Config)
	oCfg.Passthrough = true

	tp, err = factory.CreateTraceProcessor(context.Background(), params, cfg, consumertest.NewTracesNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	mp, err = factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewMetricsNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	lp, err = factory.CreateLogsProcessor(context.Background(), params, cfg, consumertest.NewLogsNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)

	// Switch it back so other tests run afterwards will not fail on unexpected state
	kubeClientProvider = realClient
}
