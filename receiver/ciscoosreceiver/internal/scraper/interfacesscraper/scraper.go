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

// interfacesScraper collects interface metrics from Cisco devices
type interfacesScraper struct {
	logger       *zap.Logger
	config       *Config
	deviceTarget string
	rpcClient    *connection.RPCClient
}

func (s *interfacesScraper) Start(_ context.Context, _ component.Host) error {
	if s.config.Device.Host.IP == "" {
		s.logger.Warn("No device configured, scraper will not collect metrics")
		return nil
	}

	device := s.config.Device
	s.deviceTarget = device.Host.IP

	s.logger.Info("Interfaces scraper initialized", zap.String("target", s.deviceTarget))

	return nil
}

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

func (s *interfacesScraper) Shutdown(_ context.Context) error {
	if s.rpcClient != nil {
		if err := s.rpcClient.SSHClient.Close(); err != nil {
			s.logger.Warn("Failed to close SSH connection", zap.Error(err))
		}
		s.rpcClient = nil
	}

	return nil
}

// parseInterfaceData establishes SSH connection and parses interface data
func (s *interfacesScraper) parseInterfaceData(ctx context.Context) ([]*Interface, error) {
	if s.rpcClient == nil {
		rpcClient, err := s.establishDeviceConnection(ctx)
		if err != nil {
			s.logger.Error("Failed to establish SSH connection", zap.String("target", s.deviceTarget), zap.Error(err))
			return []*Interface{}, fmt.Errorf("failed to establish connection: %w", err)
		}
		s.rpcClient = rpcClient
	}

	command := s.rpcClient.GetCommand("interfaces")
	if command == "" {
		return nil, fmt.Errorf("interfaces command not supported on OS type: %s", s.rpcClient.GetOSType())
	}

	output, err := s.rpcClient.ExecuteCommand(command)
	if err != nil {
		fallbackCommand := "show interface brief"
		s.logger.Warn("Primary command failed, using fallback", zap.String("fallback", fallbackCommand))
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

	return connection.EstablishConnection(ctx, deviceConfig, s.logger)
}
