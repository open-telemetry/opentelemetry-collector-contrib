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

package windowsperfcountersreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

var creationParams = component.ReceiverCreateParams{Logger: zap.NewNop()}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))

	cfg.(*Config).PerfCounters = []PerfCounterConfig{{Object: "object", Counters: []string{"counter"}}}

	assert.NoError(t, cfg.(*Config).validate())
}

func TestCreateTraceReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []PerfCounterConfig{{Object: "object", Counters: []string{"counter"}}}

	tReceiver, err := factory.CreateTracesReceiver(context.Background(), creationParams, cfg, consumertest.NewTracesNop())

	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []PerfCounterConfig{{Object: "object", Counters: []string{"counter"}}}

	tReceiver, err := factory.CreateLogsReceiver(context.Background(), creationParams, cfg, consumertest.NewLogsNop())

	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}
