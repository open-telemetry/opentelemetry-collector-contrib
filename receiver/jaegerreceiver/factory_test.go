// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/metadata"
)

func TestTypeStr(t *testing.T) {
	factory := NewFactory()

	assert.Equal(t, "jaeger", factory.Type().String())
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
	// have to enable at least one protocol for the jaeger receiver to be created
	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  "0.0.0.0:14250",
			Transport: confignet.TransportTypeTCP,
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateTraces(context.Background(), set, cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateMetrics(context.Background(), set, cfg, nil)
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, mReceiver)
}

func TestCreateReceiverGeneralConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	set := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateTraces(context.Background(), set, cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateMetrics(context.Background(), set, cfg, nil)
	assert.Equal(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, mReceiver)
}

// default ports retrieved from https://www.jaegertracing.io/docs/1.16/deployment/
func TestCreateDefaultGRPCEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  "0.0.0.0:14250",
			Transport: confignet.TransportTypeTCP,
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	r, err := factory.CreateTraces(context.Background(), set, cfg, nil)

	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, "0.0.0.0:14250", r.(*jReceiver).config.GRPC.NetAddr.Endpoint, "grpc port should be default")
}

func TestCreateTLSGPRCEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  "0.0.0.0:14250",
			Transport: confignet.TransportTypeTCP,
		},
		TLS: &configtls.ServerConfig{
			Config: configtls.Config{
				CertFile: "./testdata/server.crt",
				KeyFile:  "./testdata/server.key",
			},
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)

	_, err := factory.CreateTraces(context.Background(), set, cfg, nil)
	assert.NoError(t, err, "tls-enabled receiver creation failed")
}

func TestCreateTLSThriftHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).ThriftHTTP = &confighttp.ServerConfig{
		Endpoint: "0.0.0.0:14268",
		TLS: &configtls.ServerConfig{
			Config: configtls.Config{
				CertFile: "./testdata/server.crt",
				KeyFile:  "./testdata/server.key",
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)

	_, err := factory.CreateTraces(context.Background(), set, cfg, nil)
	assert.NoError(t, err, "tls-enabled receiver creation failed")
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := receivertest.NewNopSettings(metadata.Type)
	r, err := factory.CreateTraces(context.Background(), set, cfg, nil)

	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, "localhost:14268", r.(*jReceiver).config.ThriftHTTP.Endpoint, "http port should be default")
}

func TestCreateInvalidThriftBinaryEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).ThriftBinaryUDP = &ProtocolUDP{
		Endpoint: "0.0.0.0:6832",
	}
	set := receivertest.NewNopSettings(metadata.Type)
	r, err := factory.CreateTraces(context.Background(), set, cfg, nil)

	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, "0.0.0.0:6832", r.(*jReceiver).config.ThriftBinaryUDP.Endpoint, "thrift port should be default")
}

func TestCreateInvalidThriftCompactEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).ThriftCompactUDP = &ProtocolUDP{
		Endpoint: "0.0.0.0:6831",
	}
	set := receivertest.NewNopSettings(metadata.Type)
	r, err := factory.CreateTraces(context.Background(), set, cfg, nil)

	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, "0.0.0.0:6831", r.(*jReceiver).config.ThriftCompactUDP.Endpoint, "thrift port should be default")
}
