// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func Test_composeEffectiveConfig(t *testing.T) {
	acceptsRemoteConfig := true
	s := Supervisor{
		logger:                       zap.NewNop(),
		config:                       config.Supervisor{Capabilities: config.Capabilities{AcceptsRemoteConfig: acceptsRemoteConfig}},
		hasNewConfig:                 make(chan struct{}, 1),
		effectiveConfigFilePath:      "effective.yaml",
		agentConfigOwnMetricsSection: &atomic.Value{},
		effectiveConfig:              &atomic.Value{},
		agentHealthCheckEndpoint:     "localhost:8000",
	}

	s.agentDescription = &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "otelcol",
					},
				},
			},
		},
	}

	fileLogConfig := `
receivers:
  filelog:
    include: ['/test/logs/input.log']
    start_at: "beginning"

exporters:
  file:
    path: '/test/logs/output.log'

service:
  pipelines:
    logs:
      receivers: [filelog]
      exporters: [file]`

	require.NoError(t, s.createTemplates())
	s.loadAgentEffectiveConfig()

	configChanged, err := s.composeEffectiveConfig(&protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {
					Body: []byte(fileLogConfig),
				},
			},
		},
	})
	require.NoError(t, err)

	expectedConfig, err := os.ReadFile("../testdata/collector/effective_config.yaml")
	require.NoError(t, err)
	expectedConfig = bytes.ReplaceAll(expectedConfig, []byte("\r\n"), []byte("\n"))

	require.True(t, configChanged)
	require.Equal(t, string(expectedConfig), s.effectiveConfig.Load().(string))
}

func Test_onMessage(t *testing.T) {
	t.Run("AgentIdentification - New instance ID is valid", func(t *testing.T) {
		initialID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		newID := uuid.MustParse("018fef3f-14a8-73ef-b63e-3b96b146ea38")
		s := Supervisor{
			logger:                       zap.NewNop(),
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			effectiveConfigFilePath:      "effective.yaml",
			persistentState:              &persistentState{InstanceID: initialID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
			opampClient:                  client.NewHTTP(newLoggerFromZap(zap.NewNop())),
		}

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: newID[:],
			},
		})

		require.Equal(t, newID, s.persistentState.InstanceID)
	})

	t.Run("AgentIdentification - New instance ID is invalid", func(t *testing.T) {
		testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
		s := Supervisor{
			logger:                       zap.NewNop(),
			config:                       config.Supervisor{},
			hasNewConfig:                 make(chan struct{}, 1),
			effectiveConfigFilePath:      "effective.yaml",
			persistentState:              &persistentState{InstanceID: testUUID},
			agentConfigOwnMetricsSection: &atomic.Value{},
			effectiveConfig:              &atomic.Value{},
			agentHealthCheckEndpoint:     "localhost:8000",
		}

		s.onMessage(context.Background(), &types.MessageData{
			AgentIdentification: &protobufs.AgentIdentification{
				NewInstanceUid: []byte("invalid-value"),
			},
		})

		require.Equal(t, testUUID, s.persistentState.InstanceID)
	})
}
