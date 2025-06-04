// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "default.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	defaultCfg := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, defaultCfg, cfg)
}

func TestUnmarshalConfigOnlyGRPC(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "only_grpc.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))

	defaultOnlyGRPC := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, defaultOnlyGRPC, cfg)
}

func TestUnmarshalConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t,
		&Config{
			Protocols: Protocols{
				GRPC: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "0.0.0.0:4317",
						Transport: confignet.TransportTypeTCP,
					},
					TLS: &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "test.crt",
							KeyFile:  "test.key",
						},
					},
					MaxRecvMsgSizeMiB:    32,
					MaxConcurrentStreams: 16,
					ReadBufferSize:       1024,
					WriteBufferSize:      1024,
					Keepalive: &configgrpc.KeepaliveServerConfig{
						ServerParameters: &configgrpc.KeepaliveServerParameters{
							MaxConnectionIdle:     11 * time.Second,
							MaxConnectionAge:      12 * time.Second,
							MaxConnectionAgeGrace: 13 * time.Second,
							Time:                  30 * time.Second,
							Timeout:               5 * time.Second,
						},
						EnforcementPolicy: &configgrpc.KeepaliveEnforcementPolicy{
							MinTime:             10 * time.Second,
							PermitWithoutStream: true,
						},
					},
				},
				Arrow: ArrowConfig{
					MemoryLimitMiB: 123,
				},
			},
			Admission: AdmissionConfig{
				RequestLimitMiB: 80,
				WaitingLimitMiB: 100,
			},
		}, cfg)
}

// Tests that a deprecated config validation sets RequestLimitMiB and WaiterLimit in the correct config block.
func TestValidateDeprecatedConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "deprecated.yaml"))
	require.NoError(t, err)
	cfg := &Config{}
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.NoError(t, cfg.Validate())
	assert.Equal(t,
		&Config{
			Protocols: Protocols{
				Arrow: ArrowConfig{
					MemoryLimitMiB:              123,
					DeprecatedAdmissionLimitMiB: 80,
					DeprecatedWaiterLimit:       100,
				},
			},
			Admission: AdmissionConfig{
				// cfg.Validate should now set these fields.
				RequestLimitMiB: 80,
			},
		}, cfg)
}

func TestUnmarshalConfigUnix(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "uds.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t,
		&Config{
			Protocols: Protocols{
				GRPC: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "/tmp/grpc_otlp.sock",
						Transport: confignet.TransportTypeUnix,
					},
					ReadBufferSize: 512 * 1024,
				},
				Arrow: ArrowConfig{
					MemoryLimitMiB: defaultMemoryLimitMiB,
				},
			},
			Admission: AdmissionConfig{
				RequestLimitMiB: defaultRequestLimitMiB,
				WaitingLimitMiB: defaultWaitingLimitMiB,
			},
		}, cfg)
}

func TestUnmarshalConfigTypoDefaultProtocol(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "typo_default_proto_config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.ErrorContains(t, cm.Unmarshal(cfg), "'protocols' has invalid keys: htttp")
}

func TestUnmarshalConfigInvalidProtocol(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "bad_proto_config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.ErrorContains(t, cm.Unmarshal(cfg), "'protocols' has invalid keys: thrift")
}

func TestUnmarshalConfigNoProtocols(t *testing.T) {
	cfg := Config{}
	// This now produces an error due to breaking change.
	// https://github.com/open-telemetry/opentelemetry-collector/pull/9385
	assert.ErrorContains(t, xconfmap.Validate(cfg), "invalid transport type")
}
