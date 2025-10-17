// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

// systemScraper collects system-level metrics for the Cisco device
type systemScraper struct {
	logger          *zap.Logger
	config          *Config
	mb              *metadata.MetricsBuilder
	collectionCount int    // Track collection attempts for debugging
	deviceTarget    string // Device IP address

	sshClient *connection.Client
	rpcClient *connection.RPCClient
	connected bool // Track if connection is established and healthy
}

// Start initializes the system scraper and establishes persistent SSH connection
func (s *systemScraper) Start(_ context.Context, _ component.Host) error {
	s.logger.Info("Starting system scraper with metric configuration",
		zap.Bool("device_up_enabled", s.config.Metrics.CiscoDeviceUp.Enabled),
		zap.Bool("collector_duration_enabled", s.config.Metrics.CiscoCollectorDurationSeconds.Enabled))

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, receiver.Settings{
		ID:                component.MustNewIDWithName(metadata.Type.String(), "system"),
		TelemetrySettings: component.TelemetrySettings{Logger: s.logger},
	})

	// Initialize device target from config (Option 1: One receiver instance per device)
	if len(s.config.Devices) == 0 {
		return errors.New("no devices configured")
	}

	if len(s.config.Devices) > 1 {
		s.logger.Warn("Multiple devices configured in single receiver instance - only first device will be monitored",
			zap.Int("device_count", len(s.config.Devices)),
			zap.String("monitoring_device", s.config.Devices[0].Host.IP),
			zap.String("recommendation", "Use separate receiver instances for multiple devices (e.g., ciscoosreceiver/device1, ciscoosreceiver/device2)"))
	}

	// Use the first (and should be only) device from this receiver instance
	device := s.config.Devices[0]
	s.deviceTarget = device.Host.IP
	s.connected = false

	s.logger.Info("System scraper initialized - will establish persistent SSH connection on first collection",
		zap.String("target", s.deviceTarget),
		zap.Int("port", device.Host.Port),
		zap.String("username", device.Auth.Username))

	return nil
}

// Shutdown cleans up the system scraper and closes persistent SSH connection
func (s *systemScraper) Shutdown(_ context.Context) error {
	s.logger.Info("Shutting down system scraper")

	if s.connected && s.sshClient != nil {
		s.logger.Info("Closing persistent SSH connection", zap.String("target", s.deviceTarget))
		if s.sshClient.Connection != nil {
			if err := s.sshClient.Connection.Close(); err != nil {
				s.logger.Warn("Error closing SSH connection", zap.Error(err))
			}
		}
		s.connected = false
	}

	return nil
}

// ScrapeMetrics collects metrics using persistent SSH connection
func (s *systemScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	s.collectionCount++
	s.logger.Debug("Starting Cisco device collection using persistent connection",
		zap.Int("collection_number", s.collectionCount),
		zap.String("target", s.deviceTarget),
		zap.Bool("connected", s.connected))

	now := pcommon.NewTimestampFromTime(time.Now())
	collectionStart := time.Now()

	// Step 1: Establish or verify persistent SSH connection
	if !s.connected || s.sshClient == nil || s.rpcClient == nil {
		s.logger.Info("Establishing persistent SSH connection",
			zap.String("target", s.deviceTarget),
			zap.Int("collection_number", s.collectionCount))

		sshClient, rpcClient, err := s.establishDeviceConnection(ctx, s.deviceTarget)
		if err != nil {
			// SSH connection failed - device is down
			s.logger.Error("Device connection failed - recording cisco.device.up=0",
				zap.String("target", s.deviceTarget),
				zap.Int("collection_number", s.collectionCount),
				zap.String("error_type", fmt.Sprintf("%T", err)),
				zap.Error(err))

			s.connected = false
			s.sshClient = nil
			s.rpcClient = nil

			// Record device down status and duration metrics
			collectionDuration := time.Since(collectionStart).Seconds()
			s.mb.RecordCiscoDeviceUpDataPoint(now, 0, s.deviceTarget)
			s.mb.RecordCiscoCollectorDurationSecondsDataPoint(now, collectionDuration, s.deviceTarget)

			s.logger.Info("Device down - recorded metrics",
				zap.Float64("cisco.device.up", 0),
				zap.Float64("cisco.collector.duration.seconds", collectionDuration))

			return s.mb.Emit(), nil
		}

		// Connection successful - save for reuse
		s.sshClient = sshClient
		s.rpcClient = rpcClient
		s.connected = true

		s.logger.Info("Persistent SSH connection established successfully",
			zap.String("target", s.deviceTarget),
			zap.String("os_type", s.rpcClient.GetOSType()),
			zap.Int("collection_number", s.collectionCount))
	} else {
		s.logger.Debug("Reusing existing persistent SSH connection",
			zap.String("target", s.deviceTarget),
			zap.Int("collection_number", s.collectionCount))
	}

	// Step 2: Connection is established - device is up
	s.mb.RecordCiscoDeviceUpDataPoint(now, 1, s.deviceTarget)

	// Step 2a: Collect CPU utilization metrics
	if cpuUtil, err := s.collectCPUUtilization(ctx); err == nil {
		s.mb.RecordCiscoSystemCPUUtilizationDataPoint(now, cpuUtil)
		s.logger.Debug("Recorded CPU utilization metric",
			zap.Float64("value", cpuUtil))
	} else {
		s.logger.Warn("Failed to collect CPU utilization, skipping metric",
			zap.Error(err))
	}

	// Step 2b: Collect memory utilization metrics
	if memUtil, err := s.collectMemoryUtilization(ctx); err == nil {
		s.mb.RecordCiscoSystemMemoryUtilizationDataPoint(now, memUtil)
		s.logger.Debug("Recorded memory utilization metric",
			zap.Float64("value", memUtil))
	} else {
		s.logger.Warn("Failed to collect memory utilization, skipping metric",
			zap.Error(err))
	}

	// Step 3: Record total collector duration
	collectionDuration := time.Since(collectionStart).Seconds()
	s.mb.RecordCiscoCollectorDurationSecondsDataPoint(now, collectionDuration, s.deviceTarget)

	return s.mb.Emit(), nil
}

// establishDeviceConnection establishes SSH connection to Cisco device
func (s *systemScraper) establishDeviceConnection(ctx context.Context, target string) (*connection.Client, *connection.RPCClient, error) {
	// Get device configuration
	deviceConfig := s.getDeviceConfig(target)
	if deviceConfig == nil {
		return nil, nil, fmt.Errorf("no device configuration found for target: %s", target)
	}

	// Step 1: Create SSH client configuration
	sshConfig := &cryptossh.ClientConfig{
		User: deviceConfig.Auth.Username,
		Auth: []cryptossh.AuthMethod{
			cryptossh.Password(deviceConfig.Auth.Password),
		},
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(), // #nosec G106 - Insecure for lab/demo only
		Timeout:         10 * time.Second,
	}

	// Step 2: Establish SSH connection
	address := fmt.Sprintf("%s:%d", deviceConfig.Host.IP, deviceConfig.Host.Port)

	conn, err := cryptossh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH connection failed to %s: %w", address, err)
	}

	s.logger.Info("SSH connection established successfully", zap.String("address", address))

	// Step 3: Create SSH client wrapper
	sshClient := &connection.Client{
		Target:     address,
		Username:   deviceConfig.Auth.Username,
		Connection: conn,
		Logger:     s.logger,
	}

	// Step 4: Detect OS type using "show version" command
	s.logger.Info("Detecting Cisco OS type via 'show version' command")
	osType, err := sshClient.DetectOSType(ctx)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("OS detection failed: %w", err)
	}

	s.logger.Info("Cisco OS type detected successfully",
		zap.String("os_type", osType),
		zap.String("device", address))

	// Step 5: Create RPC client for command execution
	rpcClient := &connection.RPCClient{
		SSHClient: sshClient,
		OSType:    osType,
		Logger:    s.logger,
	}

	s.logger.Info("Device connection fully established",
		zap.String("target", address),
		zap.String("username", deviceConfig.Auth.Username),
		zap.String("detected_os", osType))

	return sshClient, rpcClient, nil
}

// getDeviceConfig returns device configuration from the scraper's config
func (s *systemScraper) getDeviceConfig(target string) *DeviceConfig {
	// Use the device config that was already validated in Start()
	// Each receiver instance has exactly one device
	if len(s.config.Devices) == 0 {
		s.logger.Error("No devices configured in getDeviceConfig")
		return nil
	}

	// Return the first (and only) device for this receiver instance
	device := &s.config.Devices[0]

	s.logger.Debug("Retrieved device config",
		zap.String("requested_target", target),
		zap.String("configured_ip", device.Host.IP),
		zap.Int("configured_port", device.Host.Port),
		zap.String("configured_username", device.Auth.Username))

	return device
}

// collectCPUUtilization collects CPU utilization metric from the device
func (s *systemScraper) collectCPUUtilization(_ context.Context) (float64, error) {
	if s.rpcClient == nil {
		return 0, errors.New("RPC client not initialized")
	}

	osType := s.rpcClient.GetOSType()
	command := s.rpcClient.GetCommand("cpu")
	if command == "" {
		return 0, fmt.Errorf("no CPU command available for OS type: %s", osType)
	}

	s.logger.Debug("Collecting CPU utilization",
		zap.String("os_type", osType),
		zap.String("command", command))

	output, err := s.rpcClient.ExecuteCommand(command)
	if err != nil {
		return 0, fmt.Errorf("failed to execute CPU command: %w", err)
	}

	// Parse based on OS type
	var cpuUtil float64
	if osType == "NX-OS" {
		cpuUtil, err = parseCPUUtilizationNXOS(output)
	} else {
		// IOS or IOS XE
		cpuUtil, err = parseCPUUtilizationIOS(output)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU utilization: %w", err)
	}

	s.logger.Debug("CPU utilization collected",
		zap.Float64("utilization", cpuUtil),
		zap.Float64("percentage", cpuUtil*100))

	return cpuUtil, nil
}

// collectMemoryUtilization collects memory utilization metric from the device
func (s *systemScraper) collectMemoryUtilization(_ context.Context) (float64, error) {
	if s.rpcClient == nil {
		return 0, errors.New("RPC client not initialized")
	}

	osType := s.rpcClient.GetOSType()
	command := s.rpcClient.GetCommand("memory")
	if command == "" {
		return 0, fmt.Errorf("no memory command available for OS type: %s", osType)
	}

	s.logger.Debug("Collecting memory utilization",
		zap.String("os_type", osType),
		zap.String("command", command))

	output, err := s.rpcClient.ExecuteCommand(command)
	if err != nil {
		return 0, fmt.Errorf("failed to execute memory command: %w", err)
	}

	// Parse based on OS type
	var memUtil float64
	if osType == "NX-OS" {
		memUtil, err = parseMemoryUtilizationNXOS(output)
	} else {
		// IOS or IOS XE
		memUtil, err = parseMemoryUtilizationIOS(output)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to parse memory utilization: %w", err)
	}

	s.logger.Debug("Memory utilization collected",
		zap.Float64("utilization", memUtil),
		zap.Float64("percentage", memUtil*100))

	return memUtil, nil
}
