// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper/internal/metadata"
)

// interfacesScraper implements scraper.Metrics interface for interface metrics collection
type interfacesScraper struct {
	logger       *zap.Logger
	config       *Config
	deviceTarget string // Device IP address from config
	rpcClient    *connection.RPCClient
}

// Start initializes the scraper
func (s *interfacesScraper) Start(_ context.Context, _ component.Host) error {
	if s.config.Device.Host.IP == "" {
		s.logger.Warn("No device configured, scraper will not collect metrics")
		return nil
	}

	device := s.config.Device
	s.deviceTarget = device.Host.IP

	authMethod := "password"
	if device.Auth.KeyFile != "" {
		authMethod = "key_file"
		if device.Auth.Password != "" {
			authMethod = "key_file+password"
		}
	}

	s.logger.Info("Interfaces scraper initialized - will establish persistent SSH connection on first collection",
		zap.String("target", s.deviceTarget),
		zap.Int("port", device.Host.Port),
		zap.String("username", device.Auth.Username),
		zap.String("auth_method", authMethod))

	return nil
}

// ScrapeMetrics implements scraper.Metrics interface
func (s *interfacesScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Panic in ScrapeMetrics", zap.Any("panic", r))
		}
	}()

	mb := metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, scraper.Settings{
		ID:                component.MustNewIDWithName(metadata.Type.String(), "interfaces"),
		TelemetrySettings: component.TelemetrySettings{Logger: s.logger},
	})

	interfaces, err := s.parseInterfaceData(ctx)
	if err != nil {
		s.logger.Error("Failed to parse interface data", zap.Error(err))
		return pmetric.NewMetrics(), err
	}

	timestamp := pcommon.NewTimestampFromTime(time.Now())

	for _, intf := range interfaces {
		macAddress := intf.MACAddress
		description := intf.Description
		speedString := intf.SpeedString
		if speedString == "" && intf.Speed > 0 {
			speedString = formatSpeed(intf.Speed)
		}

		if intf.OperStatus == "" {
			s.logger.Warn("Interface has empty OperStatus, setting to down", zap.String("interface", intf.Name))
			intf.OperStatus = StatusDown
		}

		s.logger.Debug("Recording interface metrics",
			zap.String("interface", intf.Name),
			zap.String("mac", macAddress),
			zap.String("description", description),
			zap.String("speed", speedString),
			zap.Float64("input_bytes", intf.InputBytes),
			zap.Float64("output_bytes", intf.OutputBytes),
			zap.Float64("input_errors", intf.InputErrors),
			zap.Float64("output_errors", intf.OutputErrors),
			zap.Float64("input_drops", intf.InputDrops),
			zap.Float64("output_drops", intf.OutputDrops),
			zap.Float64("multicast", intf.InputMulticast),
			zap.Float64("broadcast", intf.InputBroadcast))

		mb.RecordSystemNetworkIoDataPoint(timestamp, int64(intf.InputBytes), metadata.AttributeNetworkIoDirectionReceive, description, macAddress, intf.Name, speedString)
		mb.RecordSystemNetworkIoDataPoint(timestamp, int64(intf.OutputBytes), metadata.AttributeNetworkIoDirectionTransmit, description, macAddress, intf.Name, speedString)

		mb.RecordSystemNetworkErrorsDataPoint(timestamp, int64(intf.InputErrors), metadata.AttributeNetworkIoDirectionReceive, description, macAddress, intf.Name, speedString)
		mb.RecordSystemNetworkErrorsDataPoint(timestamp, int64(intf.OutputErrors), metadata.AttributeNetworkIoDirectionTransmit, description, macAddress, intf.Name, speedString)

		mb.RecordSystemNetworkPacketDroppedDataPoint(timestamp, int64(intf.InputDrops), metadata.AttributeNetworkIoDirectionReceive, description, macAddress, intf.Name, speedString)
		mb.RecordSystemNetworkPacketDroppedDataPoint(timestamp, int64(intf.OutputDrops), metadata.AttributeNetworkIoDirectionTransmit, description, macAddress, intf.Name, speedString)

		mb.RecordSystemNetworkPacketCountDataPoint(timestamp, int64(intf.InputMulticast), metadata.AttributeNetworkPacketTypeMulticast, description, macAddress, intf.Name, speedString)
		mb.RecordSystemNetworkPacketCountDataPoint(timestamp, int64(intf.InputBroadcast), metadata.AttributeNetworkPacketTypeBroadcast, description, macAddress, intf.Name, speedString)

		mb.RecordSystemNetworkInterfaceStatusDataPoint(timestamp, intf.GetOperStatusInt(), description, macAddress, intf.Name, speedString)
	}

	rb := mb.NewResourceBuilder()
	rb.SetHostIP(s.deviceTarget)
	rb.SetHwType("network")
	if s.rpcClient != nil {
		rb.SetOsName(s.rpcClient.GetOSType())
	}

	return mb.Emit(metadata.WithResource(rb.Emit())), nil
}

// Shutdown closes SSH connection and cleans up resources
func (s *interfacesScraper) Shutdown(_ context.Context) error {
	s.logger.Info("Shutting down interfaces scraper")

	if s.rpcClient != nil {
		s.logger.Info("Closing persistent SSH connection", zap.String("target", s.deviceTarget))
		if err := s.rpcClient.SSHClient.Close(); err != nil {
			s.logger.Warn("Error closing SSH connection", zap.Error(err))
		}
		s.rpcClient = nil
	}

	return nil
}

// parseInterfaceData establishes SSH connection and parses interface data
func (s *interfacesScraper) parseInterfaceData(ctx context.Context) ([]*Interface, error) {
	if s.rpcClient == nil {
		s.logger.Info("Establishing persistent SSH connection", zap.String("target", s.deviceTarget))
		rpcClient, err := s.establishDeviceConnection(ctx)
		if err != nil {
			s.logger.Error("Device connection failed",
				zap.String("target", s.deviceTarget),
				zap.String("error_type", fmt.Sprintf("%T", err)),
				zap.Error(err))
			return []*Interface{}, fmt.Errorf("failed to establish connection: %w", err)
		}
		s.rpcClient = rpcClient
		s.logger.Info("Persistent SSH connection established successfully",
			zap.String("target", s.deviceTarget),
			zap.String("os_type", s.rpcClient.GetOSType()))
	}

	command := s.rpcClient.GetCommand("interfaces")
	if command == "" {
		return nil, fmt.Errorf("interfaces command not supported on OS type: %s", s.rpcClient.GetOSType())
	}

	output, err := s.rpcClient.ExecuteCommand(command)
	if err != nil {
		fallbackCommand := "show interface brief"
		s.logger.Warn("Primary command failed, trying fallback", zap.String("primary", command), zap.String("fallback", fallbackCommand), zap.Error(err))
		output, err = s.rpcClient.ExecuteCommand(fallbackCommand)
		if err != nil {
			return nil, fmt.Errorf("failed to execute interface commands '%s' and '%s': %w", command, fallbackCommand, err)
		}
	}

	interfaces := parseInterfaces(output, s.logger)
	if len(interfaces) == 0 {
		s.logger.Warn("No interfaces found, trying simple parsing")
		interfaces = parseSimpleInterfaces(output, s.logger)
	}

	return interfaces, nil
}

// establishDeviceConnection establishes SSH connection to Cisco device
func (s *interfacesScraper) establishDeviceConnection(ctx context.Context) (*connection.RPCClient, error) {
	deviceConfig := connection.DeviceConfig{
		Host: connection.HostInfo{
			Name: s.config.Device.Host.Name,
			IP:   s.config.Device.Host.IP,
			Port: s.config.Device.Host.Port,
		},
		Auth: connection.AuthConfig{
			Username: s.config.Device.Auth.Username,
			Password: s.config.Device.Auth.Password,
			KeyFile:  s.config.Device.Auth.KeyFile,
		},
	}

	// Use shared connection factory
	return connection.EstablishConnection(ctx, deviceConfig, s.logger)
}
