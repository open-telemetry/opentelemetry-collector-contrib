// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/bgp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/environment"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/facts"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/interfaces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors/optics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// modularCiscoReceiver implements a modular Cisco receiver using the collector registry
type modularCiscoReceiver struct {
	config      *Config
	settings    receiver.Settings
	consumer    consumer.Metrics
	cancel      context.CancelFunc
	done        chan struct{}
	doneOnce    sync.Once
	registry    *collectors.Registry
	connections map[string]*connection.SSHClient
	mu          sync.RWMutex
}

// newModularCiscoReceiver creates a new modular Cisco receiver
func newModularCiscoReceiver(cfg *Config, settings receiver.Settings, consumer consumer.Metrics) (receiver.Metrics, error) {
	// Always validate config - tests should provide valid configs
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Create collector registry
	registry := collectors.NewRegistry()

	// Register collectors based on configuration
	if cfg.Collectors.BGP {
		registry.Register(bgp.NewCollector())
		settings.Logger.Info("Registered BGP collector")
	}
	if cfg.Collectors.Environment {
		registry.Register(environment.NewCollector())
		settings.Logger.Info("Registered Environment collector")
	}
	if cfg.Collectors.Facts {
		registry.Register(facts.NewCollector())
		settings.Logger.Info("Registered Facts collector")
	}
	if cfg.Collectors.Interfaces {
		registry.Register(interfaces.NewCollector())
		settings.Logger.Info("Registered Interfaces collector")
	}
	if cfg.Collectors.Optics {
		registry.Register(optics.NewCollector())
		settings.Logger.Info("Registered Optics collector")
	}

	return &modularCiscoReceiver{
		config:      cfg,
		settings:    settings,
		consumer:    consumer,
		done:        make(chan struct{}),
		registry:    registry,
		connections: make(map[string]*connection.SSHClient),
	}, nil
}

// Start begins the metrics collection process
func (r *modularCiscoReceiver) Start(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting modular Cisco receiver")

	r.settings.Logger.Info("Configuration loaded", zap.Int("devices", len(r.config.Devices)))

	if err := r.validateAndPrepareConnections(); err != nil {
		return fmt.Errorf("failed to validate device configurations: %w", err)
	}

	ctx, r.cancel = context.WithCancel(ctx)
	go r.runMetricsCollection(ctx)

	return nil
}

// Shutdown stops the receiver with proper connection lifecycle management
func (r *modularCiscoReceiver) Shutdown(ctx context.Context) error {
	r.settings.Logger.Info("Shutting down modular Cisco receiver")

	if r.cancel != nil {
		r.cancel()
	}

	r.cleanupAllConnections()

	// Wait for the collection goroutine to finish
	if r.done != nil {
		select {
		case <-r.done:
			r.settings.Logger.Info("Metrics collection stopped")
		case <-time.After(5 * time.Second):
			r.settings.Logger.Warn("Timeout waiting for metrics collection to stop")
		case <-ctx.Done():
			r.settings.Logger.Warn("Shutdown context cancelled")
		}
	}

	return nil
}

// runMetricsCollection runs the main collection loop
func (r *modularCiscoReceiver) runMetricsCollection(ctx context.Context) {
	defer r.doneOnce.Do(func() { close(r.done) })

	ticker := time.NewTicker(r.config.CollectionInterval)
	defer ticker.Stop()

	// Collect immediately on start
	r.collectAndSendMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.collectAndSendMetrics(ctx)
		}
	}
}

// collectAndSendMetrics collects metrics from all devices and sends them to the consumer
func (r *modularCiscoReceiver) collectAndSendMetrics(ctx context.Context) {
	allMetrics := pmetric.NewMetrics()

	// Collect from each device concurrently
	var wg sync.WaitGroup
	metricsChan := make(chan pmetric.Metrics, len(r.config.Devices))

	for _, device := range r.config.Devices {
		wg.Add(1)
		go func(dev DeviceConfig) {
			defer wg.Done()

			deviceMetrics, err := r.scrapeDevice(ctx, dev)
			if err != nil {
				r.settings.Logger.Error("Failed to scrape device", zap.String("device", dev.Host), zap.Error(err))
				return
			}

			metricsChan <- deviceMetrics
		}(device)
	}

	// Wait for all collections to complete
	go func() {
		wg.Wait()
		close(metricsChan)
	}()

	// Merge all device metrics
	for deviceMetrics := range metricsChan {
		r.mergeMetrics(allMetrics, deviceMetrics)
	}

	// Send metrics to consumer
	if allMetrics.ResourceMetrics().Len() > 0 {
		if err := r.consumer.ConsumeMetrics(ctx, allMetrics); err != nil {
			r.settings.Logger.Error("Failed to consume metrics", zap.Error(err))
		}
	}
}

// scrapeDevice collects metrics from a single Cisco device using established SSH connections
func (r *modularCiscoReceiver) scrapeDevice(ctx context.Context, device DeviceConfig) (pmetric.Metrics, error) {
	allMetrics := pmetric.NewMetrics()
	timestamp := time.Now()

	// Start timing for total collection duration
	collectionStart := time.Now()

	host := r.extractHostFromTarget(device.Host)

	r.settings.Logger.Debug("Starting device scrape",
		zap.String("host", device.Host),
		zap.String("auth_method", r.getAuthMethodName(device)))
	sshClient, err := r.getSSHConnection(device)
	if err != nil {
		r.settings.Logger.Error("Failed to establish SSH connection",
			zap.String("host", device.Host),
			zap.Error(err))
		r.addObservabilityMetric(allMetrics, internal.MetricPrefix+"up", 0, host, "", timestamp)
		r.addObservabilityMetric(allMetrics, internal.MetricPrefix+"collector_duration_seconds", time.Since(collectionStart).Seconds(), host, "", timestamp)
		return allMetrics, nil
	}

	r.addObservabilityMetric(allMetrics, internal.MetricPrefix+"up", 1, host, "", timestamp)
	r.settings.Logger.Debug("SSH connection established, creating RPC client", zap.String("host", device.Host))

	// Create RPC client with OS detection for triggering collectors
	rpcClient, err := rpc.NewClient(sshClient)
	if err != nil {
		r.settings.Logger.Error("Failed to create RPC client",
			zap.String("host", device.Host),
			zap.Error(err))
		r.addObservabilityMetric(allMetrics, internal.MetricPrefix+"collector_duration_seconds", time.Since(collectionStart).Seconds(), host, "", timestamp)
		return allMetrics, nil
	}

	// Convert device collectors config to registry format
	enabledCollectors := collectors.DeviceCollectors{
		BGP:         r.config.Collectors.BGP,
		Environment: r.config.Collectors.Environment,
		Facts:       r.config.Collectors.Facts,
		Interfaces:  r.config.Collectors.Interfaces,
		Optics:      r.config.Collectors.Optics,
	}

	r.settings.Logger.Debug("Triggering collectors for device",
		zap.String("host", device.Host),
		zap.Bool("bgp", enabledCollectors.BGP),
		zap.Bool("environment", enabledCollectors.Environment),
		zap.Bool("facts", enabledCollectors.Facts),
		zap.Bool("interfaces", enabledCollectors.Interfaces),
		zap.Bool("optics", enabledCollectors.Optics))
	collectorMetrics, collectorTimings, err := r.registry.CollectFromDeviceWithTiming(ctx, rpcClient, enabledCollectors, timestamp)
	if err != nil {
		r.settings.Logger.Error("Metrics collection failed", zap.String("device", device.Host), zap.Error(err))
	} else {
		r.settings.Logger.Debug("Collectors executed successfully",
			zap.String("host", device.Host),
			zap.Int("metrics_collected", collectorMetrics.ResourceMetrics().Len()))
	}

	// Merge collector metrics into all metrics
	if collectorMetrics.ResourceMetrics().Len() > 0 {
		r.mergeMetrics(allMetrics, collectorMetrics)
	}

	for collectorName, duration := range collectorTimings {
		r.addObservabilityMetric(allMetrics, internal.MetricPrefix+"collect_duration_seconds", duration.Seconds(), host, collectorName, timestamp)
	}
	totalDuration := time.Since(collectionStart).Seconds()
	r.addObservabilityMetric(allMetrics, internal.MetricPrefix+"collector_duration_seconds", totalDuration, host, "", timestamp)

	r.settings.Logger.Debug("Device scrape completed",
		zap.String("host", device.Host),
		zap.Float64("duration_seconds", totalDuration),
		zap.Int("total_metrics", allMetrics.ResourceMetrics().Len()))

	return allMetrics, nil
}

func (r *modularCiscoReceiver) extractHostFromTarget(target string) string {
	host := target
	if strings.Contains(target, ":") {
		d := strings.Split(target, ":")
		host = d[0]
	}
	return host
}

func (r *modularCiscoReceiver) addObservabilityMetric(metrics pmetric.Metrics, metricName string, value float64, target, collector string, timestamp time.Time) {
	rm := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes using component metadata
	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", metadata.Type.String())
	if r.settings.BuildInfo.Version != "" {
		resource.Attributes().PutStr("service.version", r.settings.BuildInfo.Version)
	}

	// Create scope metrics using metadata
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(metadata.ScopeName)
	if r.settings.BuildInfo.Version != "" {
		sm.Scope().SetVersion(r.settings.BuildInfo.Version)
	}

	// Create the metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)

	// Set metric description based on type
	switch metricName {
	case internal.MetricPrefix + "up":
		metric.SetDescription("Device reachability status")
		metric.SetUnit("1")
	case internal.MetricPrefix + "collector_duration_seconds":
		metric.SetDescription("Duration of a collector scrape for one target")
		metric.SetUnit("s")
	case internal.MetricPrefix + "collect_duration_seconds":
		metric.SetDescription("Duration of a scrape by collector and target")
		metric.SetUnit("s")
	}

	// Create gauge data point
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	dp.SetDoubleValue(value)

	// Set attributes
	dp.Attributes().PutStr("target", target)
	if collector != "" {
		dp.Attributes().PutStr("collector", collector)
	}
}

// getSSHConnection gets or creates an SSH connection for a device using the connection creation flow
func (r *modularCiscoReceiver) getSSHConnection(device DeviceConfig) (*connection.SSHClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, exists := r.connections[device.Host]; exists {
		r.settings.Logger.Debug("Reusing existing SSH connection", zap.String("host", device.Host))
		return conn, nil
	}
	conn, err := r.createConnectionForDevice(context.Background(), device)
	if err != nil {
		return nil, err
	}

	r.connections[device.Host] = conn
	r.settings.Logger.Info("SSH connection added to pool",
		zap.String("host", device.Host),
		zap.Int("total_connections", len(r.connections)))

	return conn, nil
}

// cleanupAllConnections closes all SSH connections and clears the connection pool
func (r *modularCiscoReceiver) cleanupAllConnections() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.settings.Logger.Info("Cleaning up SSH connections", zap.Int("total_connections", len(r.connections)))

	for host, conn := range r.connections {
		r.settings.Logger.Debug("Closing SSH connection", zap.String("host", host))
		if err := r.closeConnection(conn); err != nil {
			r.settings.Logger.Warn("Error closing SSH connection",
				zap.String("host", host),
				zap.Error(err))
		}
	}

	r.connections = make(map[string]*connection.SSHClient)
	r.settings.Logger.Info("All SSH connections cleaned up")
}

// closeConnection safely closes a single SSH connection
func (r *modularCiscoReceiver) closeConnection(conn *connection.SSHClient) error {
	if conn == nil {
		return nil
	}
	return nil
}

// healthCheckConnections performs health checks on existing connections
func (r *modularCiscoReceiver) healthCheckConnections() {
	r.mu.Lock()
	defer r.mu.Unlock()

	unhealthyConnections := make([]string, 0)

	for host, conn := range r.connections {
		if conn == nil {
			unhealthyConnections = append(unhealthyConnections, host)
		}
	}

	for _, host := range unhealthyConnections {
		r.settings.Logger.Warn("Removing unhealthy connection", zap.String("host", host))
		delete(r.connections, host)
	}

	if len(unhealthyConnections) > 0 {
		r.settings.Logger.Info("Connection health check completed",
			zap.Int("removed_connections", len(unhealthyConnections)),
			zap.Int("healthy_connections", len(r.connections)))
	}
}

// validateAndPrepareConnections validates device configurations and prepares connection pool
func (r *modularCiscoReceiver) validateAndPrepareConnections() error {
	r.settings.Logger.Info("Validating device configurations", zap.Int("total_devices", len(r.config.Devices)))

	for i, device := range r.config.Devices {
		if err := r.validateDeviceConnectionConfig(device, i); err != nil {
			return fmt.Errorf("device[%d] validation failed: %w", i, err)
		}

		r.settings.Logger.Info("Device configuration validated",
			zap.Int("device_index", i),
			zap.String("host", device.Host),
			zap.String("auth_method", r.getAuthMethodName(device)))
	}

	r.settings.Logger.Info("All device configurations validated successfully")
	return nil
}

// validateDeviceConnectionConfig validates a single device's connection configuration
func (r *modularCiscoReceiver) validateDeviceConnectionConfig(device DeviceConfig, index int) error {
	if device.Host == "" {
		return fmt.Errorf("host is required")
	}
	hasKeyFile := device.KeyFile != ""
	hasPassword := device.Password != "" && device.Username != ""

	if !hasKeyFile && !hasPassword {
		return fmt.Errorf("authentication method required: either key_file (Method 1) or username+password (Method 2)")
	}

	if hasKeyFile {
		if _, err := os.Stat(device.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("key_file does not exist: %s", device.KeyFile)
		}
		r.settings.Logger.Debug("SSH key file validated", zap.String("key_file", device.KeyFile))
	}
	if hasPassword {
		if device.Username == "" {
			return fmt.Errorf("username is required for password authentication")
		}
		if device.Password == "" {
			return fmt.Errorf("password is required for username authentication")
		}
		r.settings.Logger.Debug("Username/password authentication validated", zap.String("username", device.Username))
	}

	return nil
}

// getAuthMethodName returns the authentication method name for logging
func (r *modularCiscoReceiver) getAuthMethodName(device DeviceConfig) string {
	if device.KeyFile != "" {
		return "key_file"
	}
	if device.Password != "" && device.Username != "" {
		return "username_password"
	}
	return "unknown"
}

// createConnectionForDevice creates and validates SSH connection for a single device
func (r *modularCiscoReceiver) createConnectionForDevice(ctx context.Context, device DeviceConfig) (*connection.SSHClient, error) {
	r.settings.Logger.Info("Creating SSH connection",
		zap.String("host", device.Host),
		zap.String("auth_method", r.getAuthMethodName(device)))

	sshConfig := connection.SSHConfig{
		Host:     device.Host,
		Username: device.Username,
		Password: device.Password,
		KeyFile:  device.KeyFile,
		Timeout:  r.config.Timeout,
	}

	conn, err := connection.NewSSHClient(sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH connection to %s: %w", device.Host, err)
	}

	r.settings.Logger.Info("SSH connection established successfully", zap.String("host", device.Host))
	return conn, nil
}

// mergeMetrics merges source metrics into destination metrics
func (r *modularCiscoReceiver) mergeMetrics(dest, src pmetric.Metrics) {
	srcResourceMetrics := src.ResourceMetrics()
	for i := 0; i < srcResourceMetrics.Len(); i++ {
		srcRM := srcResourceMetrics.At(i)
		destRM := dest.ResourceMetrics().AppendEmpty()
		srcRM.CopyTo(destRM)
	}
}
