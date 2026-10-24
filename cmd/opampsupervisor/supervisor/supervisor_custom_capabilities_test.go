// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
)

// freePort returns an http endpoint on a TCP port that is not currently
// listening. OpAMP clients started against it never connect; the "http" scheme
// keeps client.Start asynchronous, so Start always succeeds.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// TestSupervisor_customCapabilitiesSurviveOpAMPClientReplacement pins the
// behavior from
// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/52103:
// the Collector reports its CustomCapabilities once, in its first message, so
// when the OpAMP server offers OpAMPConnectionSettings and the Supervisor
// replaces its OpAMP client, the replacement client must be seeded with the
// Collector-reported capabilities. Without that, every later custom message is
// rejected by opamp-go with ErrCustomCapabilityNotSupported and the data path
// to the server stays broken until the Collector restarts.
func TestSupervisor_customCapabilitiesSurviveOpAMPClientReplacement(t *testing.T) {
	const reportedCapability = "io.opentelemetry.supervisor.test"

	newTestSupervisor := func(t *testing.T, endpoint string, reportedCaps **protobufs.CustomCapabilities) *Supervisor {
		t.Helper()

		ctx, cancel := context.WithCancel(t.Context())

		agentDesc := &atomic.Value{}
		agentDesc.Store(&protobufs.AgentDescription{
			IdentifyingAttributes: []*protobufs.KeyValue{
				{
					Key: "service.name",
					Value: &protobufs.AnyValue{
						Value: &protobufs.AnyValue_StringValue{StringValue: "test-collector"},
					},
				},
			},
		})

		s := &Supervisor{
			runCtx:            ctx,
			runCtxCancel:      cancel,
			telemetrySettings: newNopTelemetrySettings(),
			// Replaced by startOpAMPClient when the settings offer is accepted;
			// it only serves the pre-replacement calls.
			opampClient: &mockOpAMPClient{
				setHealthFunc: func(*protobufs.ComponentHealth) {},
				setCustomCapabilitiesFunc: func(caps *protobufs.CustomCapabilities) error {
					*reportedCaps = caps
					return nil
				},
			},
			config: config.Supervisor{
				Server: config.OpAMPServer{
					Endpoint: endpoint,
				},
			},
			heartbeatIntervalSeconds:       30,
			persistentState:                &persistentState{InstanceID: uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")},
			agentDescription:               agentDesc,
			availableComponents:            &atomic.Value{},
			agentConn:                      &atomic.Value{},
			effectiveConfig:                &atomic.Value{},
			cfgState:                       &atomic.Value{},
			agentConfigOwnTelemetrySection: &atomic.Value{},
			metrics:                        &telemetry.Metrics{},
		}

		t.Cleanup(func() {
			cancel()
			stopCtx, stopCancel := context.WithTimeout(t.Context(), time.Second)
			defer stopCancel()
			_ = s.opampClient.Stop(stopCtx)
		})

		return s
	}

	t.Run("reported capabilities are re-applied to the replacement client", func(t *testing.T) {
		var reportedCaps *protobufs.CustomCapabilities
		s := newTestSupervisor(t, freePort(t), &reportedCaps)

		// The Collector reports its custom capabilities exactly once, in its
		// first message to the Supervisor.
		caps := &protobufs.CustomCapabilities{Capabilities: []string{reportedCapability}}
		s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
			CustomCapabilities: caps,
		})
		require.NotNil(t, reportedCaps, "the pre-replacement client must be told the Collector's capabilities")

		// The server offers new OpAMP connection settings (e.g. to rotate the
		// endpoint or headers); the Supervisor replaces its OpAMP client.
		require.NoError(t, s.onOpampConnectionSettings(t.Context(), &protobufs.OpAMPConnectionSettings{
			DestinationEndpoint: freePort(t),
		}))

		// The Collector later sends a custom message. opamp-go rejects it with
		// ErrCustomCapabilityNotSupported unless the replacement client was
		// seeded with the Collector-reported capabilities.
		_, err := s.opampClient.SendCustomMessage(&protobufs.CustomMessage{
			Capability: reportedCapability,
			Type:       "custom.test",
			Data:       []byte("payload"),
		})
		require.NoError(t, err,
			"a custom message must survive an OpAMP client replacement: the Collector does not re-report its capabilities")

		// The fix must not over-apply: a capability the Collector never
		// reported is still rejected.
		_, err = s.opampClient.SendCustomMessage(&protobufs.CustomMessage{
			Capability: "io.opentelemetry.supervisor.never-reported",
			Type:       "custom.test",
			Data:       []byte("payload"),
		})
		require.ErrorIs(t, err, types.ErrCustomCapabilityNotSupported,
			"capabilities the Collector never reported must stay unsupported")
	})

	t.Run("no capabilities reported means nothing to re-apply", func(t *testing.T) {
		var reportedCaps *protobufs.CustomCapabilities
		s := newTestSupervisor(t, freePort(t), &reportedCaps)

		require.NoError(t, s.onOpampConnectionSettings(t.Context(), &protobufs.OpAMPConnectionSettings{
			DestinationEndpoint: freePort(t),
		}))

		_, err := s.opampClient.SendCustomMessage(&protobufs.CustomMessage{
			Capability: reportedCapability,
			Type:       "custom.test",
			Data:       []byte("payload"),
		})
		require.ErrorIs(t, err, types.ErrCustomCapabilityNotSupported,
			"a capability that was never reported must stay unsupported")
	})
}

// TestSupervisor_customMessageSendFailureLogsDetails pins the diagnosability
// half of issue #52103: when the forward loop cannot deliver a custom message,
// the error log must carry the error and the capability and type of the
// dropped message, not a bare line.
func TestSupervisor_customMessageSendFailureLogsDetails(t *testing.T) {
	const (
		capability = "teapot"
		msgType    = "brew"
	)

	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	testUUID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")
	runCtx, runCtxCancel := context.WithCancel(t.Context())
	s := Supervisor{
		telemetrySettings: telemetrySettings{
			TelemetrySettings: component.TelemetrySettings{
				Logger:         logger,
				TracerProvider: noop.NewTracerProvider(),
			},
		},
		persistentState:                &persistentState{InstanceID: testUUID},
		agentConfigOwnTelemetrySection: &atomic.Value{},
		effectiveConfig:                &atomic.Value{},
		agentConn:                      &atomic.Value{},
		opampClient: &mockOpAMPClient{
			setHealthFunc: func(*protobufs.ComponentHealth) {},
			sendCustomMessageFunc: func(*protobufs.CustomMessage) (messageSendingChannel chan struct{}, err error) {
				return nil, types.ErrCustomCapabilityNotSupported
			},
		},
		customMessageToServer: make(chan *protobufs.CustomMessage, 10),
		doneChan:              make(chan struct{}),
		runCtx:                runCtx,
		runCtxCancel:          runCtxCancel,
	}

	loopDoneChan := make(chan struct{})
	go func() {
		defer close(loopDoneChan)
		s.forwardCustomMessagesToServerLoop()
	}()
	t.Cleanup(func() {
		close(s.doneChan)
		<-loopDoneChan
		runCtxCancel()
	})

	s.handleAgentOpAMPMessage(&mockConn{}, &protobufs.AgentToServer{
		CustomMessage: &protobufs.CustomMessage{
			Capability: capability,
			Type:       msgType,
			Data:       []byte("chamomile"),
		},
	})

	require.Eventually(t, func() bool {
		for _, entry := range logs.FilterMessage("Failed to send custom message to OpAMP server").All() {
			fields := entry.ContextMap()
			if _, hasErr := fields["error"]; !hasErr {
				continue
			}
			if gotCap, _ := fields["capability"].(string); gotCap != capability {
				continue
			}
			if gotType, _ := fields["type"].(string); gotType != msgType {
				continue
			}
			return true
		}
		return false
	}, 10*time.Second, 50*time.Millisecond,
		"a failed custom-message send must log the error, capability and type of the dropped message")
}
