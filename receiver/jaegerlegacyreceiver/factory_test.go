// Copyright 2019, OpenTelemetry Authors
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

package jaegerlegacyreceiver

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configprotocol"
	"go.uber.org/zap"
)

func TestTypeStr(t *testing.T) {
	factory := Factory{}

	assert.Equal(t, configmodels.Type("jaeger_legacy"), factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	// have to enable at least one protocol for the jaeger receiver to be created
	cfg.(*Config).Protocols[protoThriftTChannel], _ = defaultsForProtocol(protoThriftTChannel)

	tReceiver, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateMetricsReceiver(zap.NewNop(), cfg, nil)
	assert.Equal(t, err, configerror.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)
}

func TestCreateDefaultTChannelEndpoint(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Protocols[protoThriftTChannel], _ = defaultsForProtocol(protoThriftTChannel)
	r, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)

	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, 14267, r.(*jReceiver).config.CollectorThriftPort, "thrift port should be default")
}

func TestCreateNoPort(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Protocols[protoThriftTChannel] = &configprotocol.ProtocolServerSettings{
		Endpoint: "localhost:",
	}
	_, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.Error(t, err, "receiver creation with no port number must fail")
}

func TestCreateLargePort(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Protocols[protoThriftTChannel] = &configprotocol.ProtocolServerSettings{
		Endpoint: "localhost:65536",
	}
	_, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.Error(t, err, "receiver creation with too large port number must fail")
}

func TestCreateInvalidHost(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Protocols[protoThriftTChannel] = &configprotocol.ProtocolServerSettings{
		Endpoint: "1234",
	}
	_, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.Error(t, err, "receiver creation with bad hostname must fail")
}

func TestCreateNoProtocols(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Protocols = make(map[string]*configprotocol.ProtocolServerSettings)

	_, err := factory.CreateTraceReceiver(context.Background(), zap.NewNop(), cfg, nil)
	assert.Error(t, err, "receiver creation with no protocols must fail")
}

func TestCustomUnmarshalErrors(t *testing.T) {
	factory := Factory{}

	f := factory.CustomUnmarshaler()
	assert.NotNil(t, f, "custom unmarshal function should not be nil")

	err := f(viper.New(), nil)
	assert.Error(t, err, "should not have been able to marshal to a nil config")
}

func TestDefaultsForProtocolError(t *testing.T) {
	d, err := defaultsForProtocol("badproto")

	assert.Nil(t, d, "defaultsForProtocol should have returned nil")
	assert.Error(t, err, "defaultsForProtocol should have errored")
}
