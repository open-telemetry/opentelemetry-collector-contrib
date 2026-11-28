// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

const supervisorRestartHelperEnv = "OTEL_SUPERVISOR_RESTART_HELPER"

// supervisorStopMarkerEnv names a file the opamp-healthy test agent creates when it is
// interrupted, before it takes a second to exit, so that a test can act while the
// Supervisor is still stopping the agent.
const supervisorStopMarkerEnv = "OTEL_SUPERVISOR_STOP_MARKER"

// supervisorPIDFileEnv names a file the opamp test agent writes its process ID to once it
// is connected, so that a test can kill the agent the Supervisor is running.
const supervisorPIDFileEnv = "OTEL_SUPERVISOR_PID_FILE"

func TestMain(m *testing.M) {
	switch os.Getenv(supervisorRestartHelperEnv) {
	case "opamp", "opamp-healthy":
		if err := runOpAMPTestAgent(os.Getenv(supervisorRestartHelperEnv) == "opamp-healthy"); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "sleep":
		// Re-run this test binary as a stand-in Collector process so supervisor
		// restart tests can exercise real process handling without a helper script.
		time.Sleep(time.Minute)
		os.Exit(0)
	case "emit-on-interrupt":
		// Emit a shutdown log line from the child process to verify restart paths
		// clear logs produced by the intentional stop before starting again.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		_, _ = fmt.Fprint(os.Stderr, "shutdown error")
		os.Exit(0)
	}
	goleak.VerifyTestMain(m)
}

// runOpAMPTestAgent supplies bootstrap information without a Collector binary. With
// reportHealth, it reports healthy periodically and keeps running on SIGHUP, like a
// Collector that reloads its config.
func runOpAMPTestAgent(reportHealth bool) error {
	if os.Args[1] == "featuregate" {
		return nil
	}
	configBytes, err := os.ReadFile(os.Args[2])
	if err != nil {
		return err
	}
	conf := confmap.New()
	if mergeErr := config.MergeConfFromYAML(conf, configBytes); mergeErr != nil {
		return mergeErr
	}
	id, err := uuid.Parse(conf.Get("extensions::opamp::instance_uid").(string))
	if err != nil {
		return err
	}

	opampClient := client.NewWebSocket(newLoggerFromZap(zap.NewNop(), "test-agent"))
	if err := opampClient.SetAgentDescription(&protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{{
			Key:   "service.name",
			Value: &protobufs.AnyValue{Value: &protobufs.AnyValue_StringValue{StringValue: "test-agent"}},
		}},
	}); err != nil {
		return err
	}
	if err := opampClient.Start(context.Background(), types.StartSettings{
		OpAMPServerURL: conf.Get("extensions::opamp::server::ws::endpoint").(string),
		InstanceUid:    types.InstanceUid(id),
	}); err != nil {
		return err
	}
	if pidFile := os.Getenv(supervisorPIDFileEnv); pidFile != "" {
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			return err
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	if reportHealth {
		return reportHealthUntilInterrupted(opampClient, sigCh)
	}
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return opampClient.Stop(ctx)
}

// reportHealthUntilInterrupted reports healthy every 100ms until interrupted, and keeps
// running on SIGHUP. It returns without stopping the client, so that the process exits
// well within the Supervisor's stop grace period when the Supervisor restarts it.
func reportHealthUntilInterrupted(opampClient client.OpAMPClient, sigCh chan os.Signal) error {
	signal.Notify(sigCh, syscall.SIGHUP)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := opampClient.SetHealth(&protobufs.ComponentHealth{Healthy: true}); err != nil {
			return err
		}
		select {
		case sig := <-sigCh:
			if sig == os.Interrupt {
				if marker := os.Getenv(supervisorStopMarkerEnv); marker != "" {
					if err := os.WriteFile(marker, nil, 0o600); err != nil {
						return err
					}
					time.Sleep(time.Second)
				}
				return nil
			}
		case <-ticker.C:
		}
	}
}
