// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestSupervisorInstanceIDInCollectorTelemetry(t *testing.T) {
	for _, mode := range getTestModes() {
		for _, tc := range []struct {
			name       string
			enabled    bool
			instanceID string
		}{
			{name: "disabled"},
			{name: "generated", enabled: true},
			{name: "configured", enabled: true, instanceID: "22222222-2222-4222-8222-222222222222"},
		} {
			t.Run(mode.name+"/"+tc.name, func(t *testing.T) {
				backend := testbed.NewMockBackend(filepath.Join(t.TempDir(), "telemetry.json"), testbed.NewOTLPHTTPDataReceiver(4318))
				backend.EnableRecording()
				require.NoError(t, backend.Start())
				t.Cleanup(backend.Stop)

				var agentID atomic.Value
				var health atomic.Value
				var remoteStatus atomic.Value
				server := newOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, msg *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if id, err := uuid.FromBytes(msg.InstanceUid); err == nil {
							agentID.Store(id.String())
						}
						if msg.Health != nil {
							health.Store(msg.Health)
						}
						if msg.RemoteConfigStatus != nil {
							remoteStatus.Store(msg.RemoteConfigStatus)
						}
						return &protobufs.ServerToAgent{}
					},
				})
				cfgFile := getSupervisorConfig(t, "emit_telemetry", map[string]string{
					"url": server.addr, "telemetryUrl": "localhost:4318",
				})
				cfg, err := config.Load(cfgFile.Name())
				require.NoError(t, err)
				cfg.Agent.Description.IncludeSupervisorInstanceID = tc.enabled
				cfg.Agent.UseHUPConfigReload = mode.UseHUPConfigReload
				cfg.Agent.BootstrapTimeout = 10 * time.Second
				cfg.Agent.ConfigApplyTimeout = 10 * time.Second
				cfg.Agent.ConfigFiles = []string{writeTempConfigFile(t, `
receivers:
  nop:
exporters:
  nop:
service:
  telemetry:
    metrics:
      level: none
  pipelines:
    logs:
      receivers: [nop]
      exporters: [nop]
`)}
				if tc.instanceID != "" {
					cfg.Telemetry.Resource.LegacyAttributes = map[string]any{"service.instance.id": tc.instanceID}
				}
				logger, err := zap.NewDevelopment()
				require.NoError(t, err)
				s, err := supervisor.NewSupervisor(t.Context(), logger, cfg)
				require.NoError(t, err)
				defer s.Shutdown()
				require.NoError(t, s.Start(t.Context()))
				waitForSupervisorConnection(server.supervisorConnected, true)
				require.Eventually(t, func() bool {
					report, ok := health.Load().(*protobufs.ComponentHealth)
					return ok && report.Healthy
				}, 15*time.Second, 100*time.Millisecond, "Initial Collector did not become healthy")

				var supervisorID string
				require.Eventually(t, func() bool {
					for _, logs := range backend.GetReceivedLogs() {
						for i := 0; i < logs.ResourceLogs().Len(); i++ {
							attrs := logs.ResourceLogs().At(i).Resource().Attributes()
							name, _ := attrs.Get("service.name")
							if name.Str() == "opamp-supervisor" {
								id, _ := attrs.Get("service.instance.id")
								supervisorID = id.Str()
								return supervisorID != ""
							}
						}
					}
					return false
				}, 10*time.Second, 100*time.Millisecond, "Supervisor did not export its own identity")
				if tc.instanceID != "" {
					require.Equal(t, tc.instanceID, supervisorID)
				}

				// A second remote config exercises both process restart and SIGHUP reload.
				for _, version := range []string{"first", "second"} {
					conflictingLink := ""
					if tc.enabled {
						conflictingLink = "- name: supervisor.service.instance.id\n          value: wrong-supervisor-id"
					}
					remoteConfig := fmt.Sprintf(`
receivers:
  nop:
exporters:
  nop:
service:
  pipelines:
    logs:
      receivers: [nop]
      exporters: [nop]
  telemetry:
    resource:
      attributes:
        - name: service.name
          value: linked-collector
        - name: service.version
          value: %s
        - name: service.instance.id
          value: wrong-collector-id
        %s
    logs:
      processors:
        - simple:
            exporter:
              otlp:
                protocol: http/protobuf
                endpoint: http://localhost:4318
`, version, conflictingLink)
					hash := sha256.Sum256([]byte(remoteConfig))
					server.sendToSupervisor(&protobufs.ServerToAgent{RemoteConfig: &protobufs.AgentRemoteConfig{
						Config: &protobufs.AgentConfigMap{ConfigMap: map[string]*protobufs.AgentConfigObject{
							"": {Body: []byte(remoteConfig)},
						}},
						ConfigHash: hash[:],
					}})
					require.Eventually(t, func() bool {
						status, ok := remoteStatus.Load().(*protobufs.RemoteConfigStatus)
						return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED && bytes.Equal(status.LastRemoteConfigHash, hash[:])
					}, 15*time.Second, 100*time.Millisecond, "Collector did not apply config %s", version)
					require.EventuallyWithT(t, func(c *assert.CollectT) {
						found := false
						for _, logs := range backend.GetReceivedLogs() {
							for i := 0; i < logs.ResourceLogs().Len(); i++ {
								attrs := logs.ResourceLogs().At(i).Resource().Attributes()
								name, _ := attrs.Get("service.name")
								gotVersion, _ := attrs.Get("service.version")
								if name.Str() != "linked-collector" || gotVersion.Str() != version {
									continue
								}
								found = true
								id, ok := attrs.Get("service.instance.id")
								assert.True(c, ok)
								assert.Equal(c, agentID.Load(), id.Str())
								assert.NotEqual(c, supervisorID, id.Str())
								link, ok := attrs.Get("supervisor.service.instance.id")
								assert.Equal(c, tc.enabled, ok)
								if tc.enabled {
									assert.Equal(c, supervisorID, link.Str())
								}
							}
						}
						assert.True(c, found, "Collector did not export telemetry for config %s", version)
					}, 15*time.Second, 100*time.Millisecond)
				}

				if tc.name == "generated" && !mode.UseHUPConfigReload {
					badConfig, hash := createBadCollectorConf(t)
					server.sendToSupervisor(&protobufs.ServerToAgent{RemoteConfig: &protobufs.AgentRemoteConfig{
						Config: &protobufs.AgentConfigMap{ConfigMap: map[string]*protobufs.AgentConfigObject{
							"": {Body: badConfig.Bytes()},
						}},
						ConfigHash: hash,
					}})
					require.Eventually(t, func() bool {
						status, ok := remoteStatus.Load().(*protobufs.RemoteConfigStatus)
						return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED && bytes.Equal(status.LastRemoteConfigHash, hash)
					}, 15*time.Second, 100*time.Millisecond, "Collector failure was not reported")
					require.Eventually(t, func() bool {
						for _, logs := range backend.GetReceivedLogs() {
							for i := 0; i < logs.ResourceLogs().Len(); i++ {
								rl := logs.ResourceLogs().At(i)
								id, _ := rl.Resource().Attributes().Get("service.instance.id")
								if id.Str() != supervisorID {
									continue
								}
								for j := 0; j < rl.ScopeLogs().Len(); j++ {
									records := rl.ScopeLogs().At(j).LogRecords()
									for k := 0; k < records.Len(); k++ {
										if records.At(k).Body().Str() == "Agent crashed during config application, reporting FAILED status" {
											return true
										}
									}
								}
							}
						}
						return false
					}, 10*time.Second, 100*time.Millisecond, "Supervisor did not export crash log independently of Collector")
				}
			})
		}
	}
}
