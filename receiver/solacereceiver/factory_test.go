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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateTracesReceiver(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(config.NewComponentIDWithName(componentType, "primary").String())
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalReceiver(sub, cfg))

	receiver, err := factory.CreateTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumertest.NewNop(),
	)
	assert.NoError(t, err)
	castedReceiver, ok := receiver.(*solaceTracesReceiver)
	assert.True(t, ok)
	assert.Equal(t, castedReceiver.config, cfg)
}

func TestCreateTracesReceiverWrongConfig(t *testing.T) {
	factories := getTestNopFactories(t)
	factory := factories.Receivers[componentType]
	_, err := factory.CreateTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), nil, nil)
	assert.Equal(t, component.ErrDataTypeIsNotSupported, err)
}

func TestCreateTracesReceiverNilConsumer(t *testing.T) {
	factories := getTestNopFactories(t)
	cfg := createDefaultConfig()
	factory := factories.Receivers[componentType]
	_, err := factory.CreateTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, nil)
	assert.Equal(t, component.ErrNilNextConsumer, err)
}

func TestCreateTracesReceiverBadConfigNoAuth(t *testing.T) {
	factories := getTestNopFactories(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "some-queue"
	factory := factories.Receivers[componentType]
	_, err := factory.CreateTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, consumertest.NewNop())
	assert.Equal(t, errMissingAuthDetails, err)
}

func TestCreateTracesReceiverBadConfigIncompleteAuth(t *testing.T) {
	factories := getTestNopFactories(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "some-queue"
	cfg.Auth = Authentication{PlainText: &SaslPlainTextConfig{Username: "someUsername"}} // missing password
	factory := factories.Receivers[componentType]
	_, err := factory.CreateTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), cfg, consumertest.NewNop())
	assert.Equal(t, errMissingPlainTextParams, err)
}

func getTestNopFactories(t *testing.T) component.Factories {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)
	factories.Receivers[componentType] = NewFactory()
	return factories
}
