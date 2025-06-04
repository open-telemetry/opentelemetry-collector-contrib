// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver

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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver/internal/metadata"
)

func TestTypeStr(t *testing.T) {
	factory := NewFactory()

	assert.Equal(t, "skywalking", factory.Type().String())
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
	// have to enable at least one protocol for the skywalking receiver to be created
	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  "0.0.0.0:11800",
			Transport: confignet.TransportTypeTCP,
		},
	}
	traceSink := new(consumertest.TracesSink)
	set := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateTraces(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "trace receiver creation failed")
	assert.NotNil(t, tReceiver, "trace receiver creation failed")

	metricSink := new(consumertest.MetricsSink)
	mReceiver, err := factory.CreateMetrics(context.Background(), set, cfg, metricSink)
	assert.NoError(t, err, "metric receiver creation failed")
	assert.NotNil(t, mReceiver, "metric receiver creation failed")
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
	traceSink := new(consumertest.TracesSink)
	tReceiver, err := factory.CreateTraces(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "trace receiver creation failed")
	assert.NotNil(t, tReceiver, "trace receiver creation failed")

	metricSink := new(consumertest.MetricsSink)
	mReceiver, err := factory.CreateMetrics(context.Background(), set, cfg, metricSink)
	assert.NoError(t, err, "metric receiver creation failed")
	assert.NotNil(t, mReceiver, "metric receiver creation failed")
}

func TestCreateDefaultGRPCEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  "0.0.0.0:11800",
			Transport: confignet.TransportTypeTCP,
		},
	}
	traceSink := new(consumertest.TracesSink)
	set := receivertest.NewNopSettings(metadata.Type)
	r, err := factory.CreateTraces(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, 11800, r.(*sharedcomponent.SharedComponent).
		Unwrap().(*swReceiver).config.CollectorGRPCPort, "grpc port should be default")
}

func TestCreateTLSGPRCEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).GRPC = &configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  "0.0.0.0:11800",
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
	traceSink := new(consumertest.TracesSink)
	_, err := factory.CreateTraces(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "tls-enabled receiver creation failed")
}

func TestCreateTLSHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).HTTP = &confighttp.ServerConfig{
		Endpoint: "0.0.0.0:12800",
		TLS: &configtls.ServerConfig{
			Config: configtls.Config{
				CertFile: "./testdata/server.crt",
				KeyFile:  "./testdata/server.key",
			},
		},
	}

	set := receivertest.NewNopSettings(metadata.Type)
	traceSink := new(consumertest.TracesSink)
	_, err := factory.CreateTraces(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "tls-enabled receiver creation failed")
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).HTTP = &confighttp.ServerConfig{
		Endpoint: "0.0.0.0:12800",
	}
	set := receivertest.NewNopSettings(metadata.Type)
	traceSink := new(consumertest.TracesSink)
	r, err := factory.CreateTraces(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, 12800, r.(*sharedcomponent.SharedComponent).
		Unwrap().(*swReceiver).config.CollectorHTTPPort, "http port should be default")
}
