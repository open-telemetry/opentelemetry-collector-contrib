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
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories := getTestNopFactories(t)

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, len(cfg.Receivers), 2)

	// validate primary config
	primary := cfg.Receivers[config.NewComponentIDWithName(componentType, "primary")].(*Config)
	assert.NotNil(t, primary)
	assert.Equal(t, "myHost:5671", primary.Broker[0])
	assert.Equal(t, Authentication{
		PlainText: &SaslPlainTextConfig{
			Username: "otel",
			Password: "otel01$",
		},
	}, primary.Auth)
	assert.Equal(t, "queue://#trace-profile123", primary.Queue)
	assert.Equal(t, uint32(1234), primary.MaxUnacked)
	assert.Equal(t, configtls.TLSClientSetting{Insecure: false, InsecureSkipVerify: false}, primary.TLS)

	// validate backup config
	backup := cfg.Receivers[config.NewComponentIDWithName(componentType, "backup")].(*Config)
	assert.NotNil(t, backup)
	assert.Equal(t, defaultHost, backup.Broker[0])
	assert.Equal(t, Authentication{
		XAuth2: &SaslXAuth2Config{
			Username: "otel",
			Bearer:   "otel01$",
		},
	}, backup.Auth)
	assert.Equal(t, "queue://#trace-profileABC", backup.Queue)
	assert.Equal(t, defaultMaxUnaked, backup.MaxUnacked)
	assert.Equal(t, configtls.TLSClientSetting{Insecure: true, InsecureSkipVerify: false}, backup.TLS)
}

func TestLoadInvalidConfigMissingAuth(t *testing.T) {
	factories := getTestNopFactories(t)
	_, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config_invalid_noauth.yaml"), factories)
	assert.Equal(t, errMissingAuthDetails, errors.Unwrap(err))
}

func TestLoadInvalidConfigMissingQueue(t *testing.T) {
	factories := getTestNopFactories(t)
	_, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config_invalid_noqueue.yaml"), factories)
	assert.Equal(t, errMissingQueueName, errors.Unwrap(err))
}

func TestCreateTracesReceiver(t *testing.T) {
	factories := getTestNopFactories(t)
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	factory := factories.Receivers[componentType]
	rcvCfg, ok := cfg.Receivers[config.NewComponentIDWithName(componentType, "primary")]
	assert.True(t, ok)
	receiver, err := factory.CreateTracesReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(),
		rcvCfg, consumertest.NewNop())
	assert.NoError(t, err)
	castedReceiver, ok := receiver.(*solaceTracesReceiver)
	assert.True(t, ok)
	assert.Equal(t, castedReceiver.config, rcvCfg)
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
