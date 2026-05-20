// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	DefaultWeaverOTLPListenerPort = "4317/tcp"
	DefaultWeaverStopPort         = "4320/tcp"
)

var ErrOptionValidation = errors.New("weaver options failed validation")

type WeaverContext struct {
	ctx             context.Context
	weaverContainer *testcontainers.DockerContainer
	opts            *WeaverOptions
	clients         *pdataClientContext
	stopEndpoint    string
}

// NewWeaverContext returns a weaver context customized with the provided options.
func NewWeaverContext(ctx context.Context, opts *WeaverOptions) (*WeaverContext, error) {
	err := opts.validate()
	if err != nil {
		return nil, err
	}

	containerOpts := opts.testContainerOptions()

	weaverVersion := "latest"
	if opts.Version != "" {
		weaverVersion = opts.Version
	}

	weaverC, err := testcontainers.Run(
		ctx,
		fmt.Sprintf("otel/weaver:%s", weaverVersion),
		containerOpts...,
	)
	if err != nil {
		return nil, err
	}

	cleanup := func() {
		timeout := 10 * time.Second
		_ = weaverC.Stop(ctx, &timeout)
	}

	// Upon creating the container, we can gather the host and the mapped ports
	// it chose and use that to construct the pdata clients and stop endpoint.
	host, err := weaverC.Host(ctx)
	if err != nil {
		cleanup()
		return nil, err
	}
	mappedOTLPPort, err := weaverC.MappedPort(ctx, nat.Port(DefaultWeaverOTLPListenerPort))
	if err != nil {
		cleanup()
		return nil, err
	}
	mappedStopPort, err := weaverC.MappedPort(ctx, nat.Port(DefaultWeaverStopPort))
	if err != nil {
		cleanup()
		return nil, err
	}

	clients, err := newPdataClientContext(fmt.Sprintf("%s:%s", host, mappedOTLPPort.Port()))
	if err != nil {
		cleanup()
		return nil, err
	}

	return &WeaverContext{
		ctx:             ctx,
		weaverContainer: weaverC,
		opts:            opts,
		clients:         clients,
		stopEndpoint:    fmt.Sprintf("http://%s:%s/stop", host, mappedStopPort.Port()),
	}, nil
}

func (wc *WeaverContext) Shutdown() error {
	var errs []error
	if wc.clients != nil {
		errs = append(errs, wc.clients.close())
	}
	timeout := 10 * time.Second
	errs = append(errs, wc.weaverContainer.Stop(wc.ctx, &timeout))
	return errors.Join(errs...)
}

func (wc *WeaverContext) ContainerLogs() ([]string, error) {
	reader, err := wc.weaverContainer.Logs(wc.ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var logs []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

// Stop sends a POST request to Weaver's /stop endpoint to stop the listener.
// With --output=http, Weaver returns the live-check report in the response body.
func (wc *WeaverContext) Stop() ([]byte, error) {
	req, err := http.NewRequestWithContext(wc.ctx, http.MethodPost, wc.stopEndpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create stop request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
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

func (wc *WeaverContext) TestLogs(logs plog.Logs) error {
	ctx, cancel := context.WithTimeout(wc.ctx, 10*time.Second)
	defer cancel()
	_, err := wc.clients.consumeLogs(ctx, logs)
	return err
}

func (wc *WeaverContext) TestMetrics(metrics pmetric.Metrics) error {
	ctx, cancel := context.WithTimeout(wc.ctx, 10*time.Second)
	defer cancel()
	_, err := wc.clients.consumeMetrics(ctx, metrics)
	return err
}

func (wc *WeaverContext) TestTraces(traces ptrace.Traces) error {
	ctx, cancel := context.WithTimeout(wc.ctx, 10*time.Second)
	defer cancel()
	_, err := wc.clients.consumeTraces(ctx, traces)
	return err
}

type WeaverOptions struct {
	Version  string
	Registry string
}

func NewDefaultWeaverOptions() *WeaverOptions {
	return &WeaverOptions{}
}

// validate will validate the provided WeaverOptions.
// This will be called before the testcontainer options are
// constructed, meaning the construction code can assume
// valid options.
//
// TODO: This is currently not used, validation logic is not there yet
func (opts *WeaverOptions) validate() error { //nolint:revive // receiver will be used when validation logic is added
	errs := []error{}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w: %w", ErrOptionValidation, err)
	}

	return nil
}

// testContainerOptions will translate the provided
// WeaverOptions to testcontainer options.
func (opts *WeaverOptions) testContainerOptions() []testcontainers.ContainerCustomizer {
	containerOpts := []testcontainers.ContainerCustomizer{}

	// Get the command args based on the options.
	cmdArgs := opts.cmdArgs()
	if len(cmdArgs) > 0 {
		containerOpts = append(containerOpts, testcontainers.WithCmdArgs(cmdArgs...))
	}

	// Expose the required ports on the container and wait for the OTLP
	// listener to be ready before returning.
	containerOpts = append(containerOpts,
		testcontainers.WithExposedPorts(DefaultWeaverOTLPListenerPort, DefaultWeaverStopPort),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(nat.Port(DefaultWeaverOTLPListenerPort))),
	)

	return containerOpts
}

// cmdArgs constructs the weaver command args to provide to the container.
// The weaver command we want to run is `weaver registry live-check`.
// Usage docs: https://github.com/open-telemetry/weaver/blob/main/docs/usage.md#registry-live-check
func (opts *WeaverOptions) cmdArgs() []string {
	args := []string{"registry", "live-check"}
	if opts.Registry != "" {
		args = append(args, "--registry", opts.Registry)
	}
	args = append(args, "--output", "http", "--format", "json")
	return args
}

type pdataClientContext struct {
	clientConn *grpc.ClientConn
	logs       plogotlp.GRPCClient
	metrics    pmetricotlp.GRPCClient
	traces     ptraceotlp.GRPCClient
}

func newPdataClientContext(endpoint string) (*pdataClientContext, error) {
	clientConn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	clientContext := pdataClientContext{
		clientConn: clientConn,
	}
	clientContext.logs = plogotlp.NewGRPCClient(clientContext.clientConn)
	clientContext.metrics = pmetricotlp.NewGRPCClient(clientContext.clientConn)
	clientContext.traces = ptraceotlp.NewGRPCClient(clientContext.clientConn)

	return &clientContext, nil
}

func (clientCtx *pdataClientContext) consumeLogs(ctx context.Context, logs plog.Logs) (plogotlp.ExportResponse, error) {
	plogReq := plogotlp.NewExportRequestFromLogs(logs)
	return clientCtx.logs.Export(ctx, plogReq)
}

func (clientCtx *pdataClientContext) consumeMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetricotlp.ExportResponse, error) {
	pmetricReq := pmetricotlp.NewExportRequestFromMetrics(metrics)
	return clientCtx.metrics.Export(ctx, pmetricReq)
}

func (clientCtx *pdataClientContext) consumeTraces(ctx context.Context, traces ptrace.Traces) (ptraceotlp.ExportResponse, error) {
	ptraceReq := ptraceotlp.NewExportRequestFromTraces(traces)
	return clientCtx.traces.Export(ctx, ptraceReq)
}

func (clientCtx *pdataClientContext) close() error {
	return clientCtx.clientConn.Close()
}
