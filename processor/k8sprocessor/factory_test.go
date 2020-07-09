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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestFactoryType(t *testing.T) {
	f := &Factory{}
	assert.Equal(t, f.Type(), configmodels.Type("k8s_tagger"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateProcessor(t *testing.T) {
	factory := Factory{KubeClient: newFakeClient}
	cfg := factory.CreateDefaultConfig()
	params := component.ProcessorCreateParams{Logger: zap.NewNop()}

	tp, err := factory.CreateTraceProcessor(context.Background(), params, nil, cfg)
	assert.NotNil(t, tp)
	assert.NoError(t, err, "cannot create trace processor")

	mp, err := factory.CreateMetricsProcessor(context.Background(), params, nil, cfg)
	assert.NotNil(t, mp)
	assert.NoError(t, err, "cannot create metrics processor")

	oCfg := cfg.(*Config)
	oCfg.Passthrough = true

	tp, err = factory.CreateTraceProcessor(context.Background(), params, nil, cfg)
	assert.NotNil(t, tp)
	assert.NoError(t, err, "cannot create trace processor")

	mp, err = factory.CreateMetricsProcessor(context.Background(), params, nil, cfg)
	assert.NotNil(t, mp)
	assert.NoError(t, err, "cannot create metrics processor")
}
