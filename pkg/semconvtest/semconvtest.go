// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	weaverOTLPListenerPort = "4317/tcp"
	weaverStopPort         = "4320/tcp"

	sendTimeout     = 10 * time.Second
	stopTimeout     = 10 * time.Second
	shutdownTimeout = 10 * time.Second
)

// WeaverOption configures the Weaver container used for a live-check test.
type WeaverOption func(*weaverOptions)

// WithVersion selects the otel/weaver image version to use.
// Defaults to "latest"; must be v0.22.1+ (the first version with
// --output=http support).
func WithVersion(version string) WeaverOption {
	return func(o *weaverOptions) { o.version = version }
}

// WithRegistry sets the semantic convention registry passed to Weaver's
// --registry flag. When unset, Weaver uses its default registry.
func WithRegistry(registry string) WeaverOption {
	return func(o *weaverOptions) { o.registry = registry }
}

type weaverOptions struct {
	version  string
	registry string
}

// TestLogs validates the provided logs against semantic conventions using
// Weaver live-check. It starts a Weaver container, sends the logs to it over
// OTLP gRPC, and fails tb with one error per violation-level finding. The
// violation findings are returned for further inspection.
func TestLogs(tb testing.TB, logs plog.Logs, opts ...WeaverOption) []PolicyFinding {
	tb.Helper()
	return runLiveCheck(tb, opts, func(ctx context.Context, clients *pdataClients) error {
		_, err := clients.consumeLogs(ctx, logs)
		return err
	})
}

// TestMetrics validates the provided metrics against semantic conventions
// using Weaver live-check. It starts a Weaver container, sends the metrics to
// it over OTLP gRPC, and fails tb with one error per violation-level finding.
// The violation findings are returned for further inspection.
func TestMetrics(tb testing.TB, metrics pmetric.Metrics, opts ...WeaverOption) []PolicyFinding {
	tb.Helper()
	return runLiveCheck(tb, opts, func(ctx context.Context, clients *pdataClients) error {
		_, err := clients.consumeMetrics(ctx, metrics)
		return err
	})
}

// TestTraces validates the provided traces against semantic conventions using
// Weaver live-check. It starts a Weaver container, sends the traces to it over
// OTLP gRPC, and fails tb with one error per violation-level finding. The
// violation findings are returned for further inspection.
func TestTraces(tb testing.TB, traces ptrace.Traces, opts ...WeaverOption) []PolicyFinding {
	tb.Helper()
	return runLiveCheck(tb, opts, func(ctx context.Context, clients *pdataClients) error {
		_, err := clients.consumeTraces(ctx, traces)
		return err
	})
}

// runLiveCheck performs the full live-check lifecycle: start a Weaver
// container, send telemetry, stop the listener, parse the report, and fail tb
// for every violation-level finding. The violation findings are returned.
func runLiveCheck(tb testing.TB, opts []WeaverOption, send func(context.Context, *pdataClients) error) []PolicyFinding {
	tb.Helper()

	options := &weaverOptions{version: "latest"}
	for _, opt := range opts {
		opt(options)
	}

	ctx := tb.Context()

	session, err := startWeaver(ctx, options)
	if err != nil {
		tb.Fatalf("failed to start weaver container: %v", err)
	}
	tb.Cleanup(func() {
		// tb.Context() is already canceled by the time cleanup functions
		// run, so shutdown gets a non-cancelable context.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if shutdownErr := session.shutdown(shutdownCtx); shutdownErr != nil {
			tb.Errorf("failed to shut down weaver container: %v", shutdownErr)
		}
	})

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if sendErr := send(sendCtx, session.clients); sendErr != nil {
		tb.Fatalf("failed to send telemetry to weaver: %v", sendErr)
	}

	content, err := session.stop(ctx)
	if err != nil {
		tb.Fatalf("failed to stop weaver listener: %v", err)
	}

	report, err := ParseLiveCheckReport(content)
	if err != nil {
		tb.Fatalf("failed to parse live-check report: %v", err)
	}

	violations := report.GetViolations()
	for _, violation := range violations {
		tb.Errorf("semantic convention violation [%s]: %s", violation.ID, violation.Message)
	}

	// Safety net: Weaver's own statistics count violation-level findings
	// independently of our sample traversal. If the statistics report
	// violations that the traversal did not surface (e.g. inside a sample
	// type this package does not recognize yet), fail rather than pass
	// silently.
	if len(violations) == 0 && report.HasViolations() {
		tb.Errorf("live-check statistics report violation-level findings, but none were found in the samples; the report may contain a sample type unknown to this package")
	}

	return violations
}

// weaverSession holds the running Weaver container and the clients used to
// talk to it.
type weaverSession struct {
	container    *testcontainers.DockerContainer
	clients      *pdataClients
	stopEndpoint string
}

// startWeaver starts a Weaver live-check container and constructs the OTLP
// clients and stop endpoint from the container's mapped ports.
func startWeaver(ctx context.Context, opts *weaverOptions) (*weaverSession, error) {
	container, err := testcontainers.Run(
		ctx,
		fmt.Sprintf("otel/weaver:%s", opts.version),
		opts.testContainerOptions()...,
	)
	if err != nil {
		return nil, err
	}

	cleanup := func() {
		timeout := shutdownTimeout
		_ = container.Stop(ctx, &timeout)
	}

	host, err := container.Host(ctx)
	if err != nil {
		cleanup()
		return nil, err
	}
	mappedOTLPPort, err := container.MappedPort(ctx, nat.Port(weaverOTLPListenerPort))
	if err != nil {
		cleanup()
		return nil, err
	}
	mappedStopPort, err := container.MappedPort(ctx, nat.Port(weaverStopPort))
	if err != nil {
		cleanup()
		return nil, err
	}

	clients, err := newPdataClients(fmt.Sprintf("%s:%s", host, mappedOTLPPort.Port()))
	if err != nil {
		cleanup()
		return nil, err
	}

	return &weaverSession{
		container:    container,
		clients:      clients,
		stopEndpoint: fmt.Sprintf("http://%s:%s/stop", host, mappedStopPort.Port()),
	}, nil
}

// stop sends a POST request to Weaver's /stop endpoint to stop the listener.
// With --output=http, Weaver returns the live-check report in the response
// body.
func (s *weaverSession) stop(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, stopTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.stopEndpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create stop request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call stop endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stop endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read stop response: %w", err)
	}

	return body, nil
}

// shutdown closes the OTLP clients and stops the Weaver container.
func (s *weaverSession) shutdown(ctx context.Context) error {
	var errs []error
	if s.clients != nil {
		errs = append(errs, s.clients.close())
	}
	timeout := shutdownTimeout
	errs = append(errs, s.container.Stop(ctx, &timeout))
	return errors.Join(errs...)
}

// testContainerOptions translates the weaver options to testcontainer
// options.
func (opts *weaverOptions) testContainerOptions() []testcontainers.ContainerCustomizer {
	containerOpts := []testcontainers.ContainerCustomizer{
		testcontainers.WithCmdArgs(opts.cmdArgs()...),
		// Expose the required ports on the container and wait for the OTLP
		// listener to be ready before returning.
		testcontainers.WithExposedPorts(weaverOTLPListenerPort, weaverStopPort),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(nat.Port(weaverOTLPListenerPort))),
	}
	return containerOpts
}

// cmdArgs constructs the weaver command args to provide to the container.
// The weaver command we want to run is `weaver registry live-check`.
// Usage docs: https://github.com/open-telemetry/weaver/blob/main/docs/usage.md#registry-live-check
func (opts *weaverOptions) cmdArgs() []string {
	args := []string{"registry", "live-check"}
	if opts.registry != "" {
		args = append(args, "--registry", opts.registry)
	}
	args = append(args, "--output", "http", "--format", "json")
	return args
}

type pdataClients struct {
	clientConn *grpc.ClientConn
	logs       plogotlp.GRPCClient
	metrics    pmetricotlp.GRPCClient
	traces     ptraceotlp.GRPCClient
}

func newPdataClients(endpoint string) (*pdataClients, error) {
	clientConn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	clients := pdataClients{
		clientConn: clientConn,
	}
	clients.logs = plogotlp.NewGRPCClient(clients.clientConn)
	clients.metrics = pmetricotlp.NewGRPCClient(clients.clientConn)
	clients.traces = ptraceotlp.NewGRPCClient(clients.clientConn)

	return &clients, nil
}

func (clients *pdataClients) consumeLogs(ctx context.Context, logs plog.Logs) (plogotlp.ExportResponse, error) {
	plogReq := plogotlp.NewExportRequestFromLogs(logs)
	return clients.logs.Export(ctx, plogReq)
}

func (clients *pdataClients) consumeMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetricotlp.ExportResponse, error) {
	pmetricReq := pmetricotlp.NewExportRequestFromMetrics(metrics)
	return clients.metrics.Export(ctx, pmetricReq)
}

func (clients *pdataClients) consumeTraces(ctx context.Context, traces ptrace.Traces) (ptraceotlp.ExportResponse, error) {
	ptraceReq := ptraceotlp.NewExportRequestFromTraces(traces)
	return clients.traces.Export(ctx, ptraceReq)
}

func (clients *pdataClients) close() error {
	return clients.clientConn.Close()
}
