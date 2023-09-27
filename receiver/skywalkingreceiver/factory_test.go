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

	assert.Equal(t, "skywalking", string(factory.Type()))
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
	cfg.(*Config).Protocols.GRPC = &configgrpc.GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  defaultGRPCBindEndpoint,
			Transport: "tcp",
		},
	}
	traceSink := new(consumertest.TracesSink)
	set := receivertest.NewNopCreateSettings()
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "trace receiver creation failed")
	assert.NotNil(t, tReceiver, "trace receiver creation failed")

	metricSink := new(consumertest.MetricsSink)
	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), set, cfg, metricSink)
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	set := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "trace receiver creation failed")
	assert.NotNil(t, tReceiver, "trace receiver creation failed")

	metricSink := new(consumertest.MetricsSink)
	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), set, cfg, metricSink)
	assert.NoError(t, err, "metric receiver creation failed")
	assert.NotNil(t, mReceiver, "metric receiver creation failed")
}

func TestCreateDefaultGRPCEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).Protocols.GRPC = &configgrpc.GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  defaultGRPCBindEndpoint,
			Transport: "tcp",
		},
	}
	traceSink := new(consumertest.TracesSink)
	set := receivertest.NewNopCreateSettings()
	r, err := factory.CreateTracesReceiver(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, 11800, r.(*sharedcomponent.SharedComponent).
		Unwrap().(*swReceiver).config.CollectorGRPCPort, "grpc port should be default")
}

func TestCreateTLSGPRCEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).Protocols.GRPC = &configgrpc.GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  defaultGRPCBindEndpoint,
			Transport: "tcp",
		},
		TLSSetting: &configtls.TLSServerSetting{
			TLSSetting: configtls.TLSSetting{
				CertFile: "./testdata/server.crt",
				KeyFile:  "./testdata/server.key",
			},
		},
	}
	set := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)
	_, err := factory.CreateTracesReceiver(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "tls-enabled receiver creation failed")
}

func TestCreateTLSHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).Protocols.HTTP = &confighttp.HTTPServerSettings{
		Endpoint: defaultHTTPBindEndpoint,
		TLSSetting: &configtls.TLSServerSetting{
			TLSSetting: configtls.TLSSetting{
				CertFile: "./testdata/server.crt",
				KeyFile:  "./testdata/server.key",
			},
		},
	}

	set := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)
	_, err := factory.CreateTracesReceiver(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "tls-enabled receiver creation failed")
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cfg.(*Config).Protocols.HTTP = &confighttp.HTTPServerSettings{
		Endpoint: defaultHTTPBindEndpoint,
	}
	set := receivertest.NewNopCreateSettings()
	traceSink := new(consumertest.TracesSink)
	r, err := factory.CreateTracesReceiver(context.Background(), set, cfg, traceSink)
	assert.NoError(t, err, "unexpected error creating receiver")
	assert.Equal(t, 12800, r.(*sharedcomponent.SharedComponent).
		Unwrap().(*swReceiver).config.CollectorHTTPPort, "http port should be default")
}
