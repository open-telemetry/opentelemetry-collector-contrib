package semconvtest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/fsnotify/fsnotify"
	"github.com/testcontainers/testcontainers-go"
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
	containerOutputDir            = "/output"
)

var (
	ErrOptionValidation = errors.New("weaver options failed validation")
)

type WeaverContext struct {
	ctx             context.Context
	weaverContainer *testcontainers.DockerContainer
	opts            *WeaverOptions
	clients         *pdataClientContext
	outputDir       string
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

	// Upon creating the container, we can gather the host and the mapped ports
	// it chose and use that to construct the pdata clients and stop endpoint.
	host, err := weaverC.Host(ctx)
	if err != nil {
		return nil, err
	}
	mappedOTLPPort, err := weaverC.MappedPort(ctx, nat.Port(DefaultWeaverOTLPListenerPort))
	if err != nil {
		return nil, err
	}
	mappedStopPort, err := weaverC.MappedPort(ctx, nat.Port(DefaultWeaverStopPort))
	if err != nil {
		return nil, err
	}

	clients, err := newPdataClientContext(fmt.Sprintf("%s:%s", host, mappedOTLPPort.Port()))
	if err != nil {
		return nil, err
	}

	return &WeaverContext{
		ctx:             ctx,
		weaverContainer: weaverC,
		opts:            opts,
		clients:         clients,
		outputDir:       opts.OutputDir,
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
	logs := []string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}
	return logs, nil
}

// Stop sends a POST request to Weaver's /stop endpoint to stop the listener
// and trigger writing of the output file.
func (wc *WeaverContext) Stop() error {
	req, err := http.NewRequestWithContext(wc.ctx, http.MethodPost, wc.stopEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create stop request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call stop endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stop endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

// WaitForOutput watches the output directory for the live_check.json file
// to be created and returns its path. This should be called after Stop().
func (wc *WeaverContext) WaitForOutput(timeout time.Duration) (string, error) {
	if wc.outputDir == "" {
		return "", errors.New("outputDir not configured")
	}

	outputFile := filepath.Join(wc.outputDir, "live_check.json")

	// Check if file already exists with valid JSON before watching
	if data, err := os.ReadFile(outputFile); err == nil && json.Valid(data) {
		return outputFile, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return "", fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	err = watcher.Add(wc.outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to watch directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(wc.ctx, timeout)
	defer cancel()

	fileCreated := false
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return "", errors.New("watcher closed")
			}
			if event.Name == outputFile {
				if event.Op.Has(fsnotify.Create) {
					fileCreated = true
				}
				// After file is created, wait for Write events and validate JSON is complete
				if fileCreated && event.Op.Has(fsnotify.Write) {
					if data, err := os.ReadFile(outputFile); err == nil && json.Valid(data) {
						return outputFile, nil
					}
					// Keep waiting for more Write events if JSON is not yet valid
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return "", errors.New("watcher closed")
			}
			return "", fmt.Errorf("watcher error: %w", err)
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for output file: %w", ctx.Err())
		}
	}
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
	Version   string
	Registry  string
	OutputDir string
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
func (opts *WeaverOptions) validate() error {
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

	// Expose the required ports on the container.
	containerOpts = append(containerOpts, testcontainers.WithExposedPorts(DefaultWeaverOTLPListenerPort, DefaultWeaverStopPort))

	// If OutputDir is set, bind mount it into the container.
	if opts.OutputDir != "" {
		containerOpts = append(containerOpts, testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds, fmt.Sprintf("%s:%s", opts.OutputDir, containerOutputDir))
		}))
	}

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

	if opts.OutputDir != "" {
		args = append(args, "--output", containerOutputDir)
	}
	args = append(args, "--format", "json")
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
