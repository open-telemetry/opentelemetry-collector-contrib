// Copyright 2020, OpenTelemetry Authors
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

package jmxmetricsextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestWithInvalidConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, configmodels.Type("jmx_metrics"), f.Type())

	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)

	r, err := f.CreateExtension(
		context.Background(),
		component.ExtensionCreateParams{Logger: zap.NewNop()},
		cfg,
	)
	require.Error(t, err)
	assert.Equal(t, "jmx_metrics missing required fields: `service_url`, `target_system` or `groovy_script`", err.Error())
	require.Nil(t, r)
}

func TestWithValidConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, configmodels.Type("jmx_metrics"), f.Type())

	cfg := f.CreateDefaultConfig()
	cfg.(*config).ServiceURL = "myserviceurl"
	cfg.(*config).GroovyScript = "mygroovyscriptpath"

	params := component.ExtensionCreateParams{Logger: zap.NewNop()}
	r, err := f.CreateExtension(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, r)
	extension := r.(*jmxMetricsExtension)
	assert.Same(t, extension.logger, params.Logger)
	assert.Same(t, extension.config, cfg)
}
