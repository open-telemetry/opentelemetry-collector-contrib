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

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
)

func TestCreateTracesReceiver(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "primary").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	receiver, err := factory.CreateTracesReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	assert.NoError(t, err)
	castedReceiver, ok := receiver.(*solaceTracesReceiver)
	assert.True(t, ok)
	assert.Equal(t, castedReceiver.config, cfg)
}

func TestCreateTracesReceiverWrongConfig(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), nil, nil)
	assert.Equal(t, component.ErrDataTypeIsNotSupported, err)
}

func TestCreateTracesReceiverNilConsumer(t *testing.T) {
	cfg := createDefaultConfig()
	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	assert.Equal(t, component.ErrNilNextConsumer, err)
}

func TestCreateTracesReceiverBadConfigNoAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "some-queue"
	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Equal(t, errMissingAuthDetails, err)
}

func TestCreateTracesReceiverBadConfigIncompleteAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "some-queue"
	cfg.Auth = Authentication{PlainText: &SaslPlainTextConfig{Username: "someUsername"}} // missing password
	factory := NewFactory()
	_, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Equal(t, errMissingPlainTextParams, err)
}

func TestCreateTracesReceiverBadMetrics(t *testing.T) {
	// register a metric first with the same name
	statName := "solacereceiver/primary/failed_reconnections"
	stat := stats.Int64(statName, "", stats.UnitDimensionless)
	err := view.Register(&view.View{
		Name:        buildReceiverCustomMetricName(statName),
		Description: "some description",
		Measure:     stat,
		Aggregation: view.Sum(),
	})
	require.NoError(t, err)

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "primary").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	receiver, err := factory.CreateTracesReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	assert.Error(t, err)
	assert.Nil(t, receiver)
}
