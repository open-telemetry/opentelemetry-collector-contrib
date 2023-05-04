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

package receivercreator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := createDefaultConfig()

	params := receivertest.NewNopCreateSettings()
	tReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	tReceiver, err = factory.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)

	lReceiver, err := factory.CreateLogsReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, lReceiver)
}
