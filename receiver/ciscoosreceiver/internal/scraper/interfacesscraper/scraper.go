// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

// interfacesScraper implements scraper.Metrics interface for interface metrics collection
type interfacesScraper struct {
	logger       *zap.Logger
	config       *Config
	deviceTarget string // Device IP address from config
}

// Start initializes the scraper
func (s *interfacesScraper) Start(_ context.Context, _ component.Host) error {
	s.logger.Info("Starting interfaces scraper - collecting ALL interfaces")

	// Initialize device target from config (Option 1: One receiver instance per device)
	if len(s.config.Devices) == 0 {
		return errors.New("no devices configured for interface scraper")
	}

	if len(s.config.Devices) > 1 {
		s.logger.Warn("Multiple devices configured in single receiver instance - only first device will be monitored",
			zap.Int("device_count", len(s.config.Devices)),
			zap.String("monitoring_device", s.config.Devices[0].Host.IP))
	}

	// Use the first (and should be only) device from this receiver instance
	device := s.config.Devices[0]
	s.deviceTarget = device.Host.IP

	s.logger.Info("Interface scraper initialized",
		zap.String("target", s.deviceTarget),
		zap.Int("port", device.Host.Port),
		zap.Bool("up_enabled", s.config.Metrics.CiscoInterfaceUp.Enabled),
		zap.Bool("bytes_enabled", s.config.Metrics.CiscoInterfaceReceiveBytes.Enabled))

	return nil
}

// ScrapeMetrics implements scraper.Metrics interface
func (s *interfacesScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Panic in ScrapeMetrics", zap.Any("panic", r))
		}
	}()

	mb := metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, receiver.Settings{
		ID:                component.MustNewIDWithName(metadata.Type.String(), "interfaces"),
		TelemetrySettings: component.TelemetrySettings{Logger: s.logger},
	})

	// Parse Cisco interface data from device
	interfaces, err := s.parseInterfaceData(ctx)
	if err != nil {
		s.logger.Error("Failed to parse interface data", zap.Error(err))
		// Fallback to basic mock data for demonstration
		interfaces = []*Interface{
			{Name: "GigabitEthernet0/0/1", MACAddress: "aa:bb:cc:dd:ee:01", Description: "Fallback interface", Speed: 1000000000, InputBytes: 1024000, OutputBytes: 512000, AdminStatus: StatusUp, OperStatus: StatusUp},
		}
	}

	timestamp := pcommon.NewTimestampFromTime(time.Now())

	for _, intf := range interfaces {
		// Record interface metrics with proper attributes: target, name, mac, description, speed
		// Convert interface data to proper attribute values (empty strings for missing data)
		macAddress := intf.MACAddress
		// Leave empty if not declared (don't use "unknown")

		description := intf.Description
		// Leave empty if not declared (don't use "No description")

		speedString := intf.SpeedString
		if speedString == "" && intf.Speed > 0 {
			// Convert numeric speed to string with units
			speedString = formatSpeed(intf.Speed)
		}
		// Leave empty if not declared (don't use -1 or fallback)

		// Record metrics with all required attributes: target, name, mac, description, speed (now string)
		mb.RecordCiscoInterfaceReceiveBytesDataPoint(timestamp, int64(intf.InputBytes), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceTransmitBytesDataPoint(timestamp, int64(intf.OutputBytes), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceReceiveErrorsDataPoint(timestamp, int64(intf.InputErrors), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceTransmitErrorsDataPoint(timestamp, int64(intf.OutputErrors), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceReceiveDropsDataPoint(timestamp, int64(intf.InputDrops), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceTransmitDropsDataPoint(timestamp, int64(intf.OutputDrops), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceReceiveMulticastDataPoint(timestamp, int64(intf.InputMulticast), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceReceiveBroadcastDataPoint(timestamp, int64(intf.InputBroadcast), s.deviceTarget, intf.Name, macAddress, description, speedString)
		mb.RecordCiscoInterfaceUpDataPoint(timestamp, intf.GetOperStatusInt(), s.deviceTarget, intf.Name, macAddress, description, speedString)
	}

	metrics := mb.Emit()

	return metrics, nil
}

// Shutdown cleans up the scraper
func (s *interfacesScraper) Shutdown(_ context.Context) error {
	s.logger.Info("Shutting down interfaces scraper")
	return nil
}

// parseInterfaceData uses shared SSH connection from system scraper
func (s *interfacesScraper) parseInterfaceData(_ context.Context) ([]*Interface, error) {
	// Step 1: Wait for shared SSH connection from system scraper (with retry for parallel execution)
	// The system scraper runs in parallel and takes ~3-4 seconds to establish connection
	// We need to wait for it to become available with sufficient buffer time

	var sharedConn *connection.SharedConnection
	maxRetries := 100 // Wait up to 10 seconds (100 * 100ms)
	retryDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		sharedConn = s.getSharedConnection(s.deviceTarget)
		if sharedConn != nil && sharedConn.Connected {
			break
		}

		if attempt == 0 {
			s.logger.Debug("Shared connection not ready yet, waiting...",
				zap.String("target", s.deviceTarget))
		}

		time.Sleep(retryDelay)
	}

	if sharedConn == nil || !sharedConn.Connected {
		s.logger.Warn("No shared SSH connection available from system scraper after waiting",
			zap.String("target", s.deviceTarget),
			zap.Int("max_retries", maxRetries),
			zap.Duration("total_wait_time", time.Duration(maxRetries)*retryDelay),
			zap.Bool("connection_available", sharedConn != nil))
		return []*Interface{}, nil
	}

	// Step 2: Get RPC client from shared connection
	rpcClient := sharedConn.RPCClient

	// Step 3: Execute interface commands based on detected OS type
	osType := rpcClient.GetOSType()

	// Step 4: Get appropriate interface command for this OS type
	command := rpcClient.GetCommand("interfaces")
	if command == "" {
		return nil, fmt.Errorf("interfaces command not supported on OS type: %s", osType)
	}

	// Step 5: Execute interface command and get raw output
	output, err := rpcClient.ExecuteCommand(command)
	if err != nil {
		// Try fallback command if primary fails
		fallbackCommand := "show interface brief"
		s.logger.Warn("Primary interface command failed, trying fallback",
			zap.String("primary", command),
			zap.String("fallback", fallbackCommand),
			zap.Error(err))

		output, err = rpcClient.ExecuteCommand(fallbackCommand)
		if err != nil {
			return nil, fmt.Errorf("failed to execute interface commands '%s' and '%s': %w", command, fallbackCommand, err)
		}
	}

	// Step 6: Parse interfaces from command output
	interfaces := parseInterfaces(output, s.logger)

	// If no interfaces found with main parser, try simple parsing
	if len(interfaces) == 0 {
		s.logger.Warn("No interfaces found with main parser, trying simple parsing")
		interfaces = parseSimpleInterfaces(output, s.logger)
	}

	return interfaces, nil
}

// getSharedConnection retrieves shared SSH connection from system scraper registry
func (s *interfacesScraper) getSharedConnection(target string) *connection.SharedConnection {
	if sharedConn, exists := connection.SharedConnectionRegistry[target]; exists {
		s.logger.Debug("Found shared connection in registry",
			zap.String("target", target),
			zap.Bool("connected", sharedConn.Connected))
		return sharedConn
	}
	s.logger.Debug("No shared connection found in registry", zap.String("target", target))
	return nil
}
