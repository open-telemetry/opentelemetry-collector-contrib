// Copyright  OpenTelemetry Authors
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

package awscontainerinsightprocessor

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	params := component.ProcessorCreateParams{Logger: zap.NewNop()}

	mp, err := factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)

	originalHostName := os.Getenv("HOST_NAME")
	os.Setenv("HOST_NAME", "host_name")
	mp, err = factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)
	os.Setenv("HOST_NAME", originalHostName)

	originalHostIP := os.Getenv("HOST_IP")
	os.Setenv("HOST_IP", "host_ip")
	mp, err = factory.CreateMetricsProcessor(context.Background(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, mp)
	assert.NoError(t, err)
	err = mp.ConsumeMetrics(context.Background(), pdata.Metrics{})
	assert.NoError(t, err)
	os.Setenv("HOST_IP", originalHostIP)
}
