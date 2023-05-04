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

package jmxreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestWithValidConfig(t *testing.T) {
	mockJarVersions()
	defer unmockJarVersions()

	f := NewFactory()
	assert.Equal(t, component.Type("jmx"), f.Type())

	cfg := f.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "myendpoint:12345"
	cfg.(*Config).JARPath = "testdata/fake_jmx.jar"
	cfg.(*Config).TargetSystem = "jvm"

	params := receivertest.NewNopCreateSettings()
	r, err := f.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	receiver := r.(*jmxMetricReceiver)
	assert.Same(t, receiver.logger, params.Logger)
	assert.Same(t, receiver.config, cfg)
}
