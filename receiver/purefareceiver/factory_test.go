// Copyright 2022 The OpenTelemetry Authors
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

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestTypeStr(t *testing.T) {
	factory := NewFactory()

	assert.Equal(t, "purefa", string(factory.Type()))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopCreateSettings()
	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), set, cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")

	tReceiver, err := factory.CreateTracesReceiver(context.Background(), set, cfg, nil)
	assert.Equal(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}

/* func TestCreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	_, err := NewFactory().CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		nil,
	)
	require.NoError(t, err)
} */

func TestCreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	//cfg.Arrays = []string{"invalid:9092"}
	f := purefaReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	// no available broker
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestCreateLogsReceiver_error(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker at startup
	f := purefaReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestWithLogsUnmarshalers(t *testing.T) {
	unmarshaler := &customLogsUnmarshaler{}
	f := NewFactory(WithLogsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker

	t.Run("custom_encoding", func(t *testing.T) {
		exporter, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, exporter)
	})
	t.Run("default_encoding", func(t *testing.T) {
		exporter, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, exporter)
	})
}

type customLogsUnmarshaler struct {
}

func (c customLogsUnmarshaler) Unmarshal([]byte) (plog.Logs, error) {
	panic("implement me")
}

func (c customLogsUnmarshaler) Encoding() string {
	return "custom"
}
