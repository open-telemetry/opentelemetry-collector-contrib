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
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/systemscraper/internal/metadata"
)

// systemScraper collects system-level metrics for the Cisco device
type systemScraper struct {
	logger          *zap.Logger
	config          *Config
	mb              *metadata.MetricsBuilder
	collectionCount int
	deviceTarget    string
	rpcClient       *connection.RPCClient
}

// Start initializes the scraper and establishes SSH connection
func (s *systemScraper) Start(_ context.Context, _ component.Host) error {
	s.logger.Info("Starting system scraper with metric configuration",
		zap.Bool("device_up_enabled", s.config.Metrics.CiscoDeviceUp.Enabled))

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, scraper.Settings{
		ID:                component.MustNewIDWithName(metadata.Type.String(), "system"),
		TelemetrySettings: component.TelemetrySettings{Logger: s.logger},
	})

	if s.config.Device.Host.IP == "" {
		return errors.New("no device configured")
	}

	device := s.config.Device
	s.deviceTarget = device.Host.IP

	// Log authentication method
	authMethod := "password"
	if device.Auth.KeyFile != "" {
		authMethod = "key_file"
		if device.Auth.Password != "" {
			authMethod = "key_file+password"
		}
	}

	s.logger.Info("System scraper initialized - will establish persistent SSH connection on first collection",
		zap.String("target", s.deviceTarget),
		zap.Int("port", device.Host.Port),
		zap.String("username", device.Auth.Username),
		zap.String("auth_method", authMethod))

	return nil
}

// Shutdown closes the SSH connection
func (s *systemScraper) Shutdown(_ context.Context) error {
	s.logger.Info("Shutting down system scraper")

	if s.rpcClient != nil {
		s.logger.Info("Closing persistent SSH connection", zap.String("target", s.deviceTarget))
		if err := s.rpcClient.SSHClient.Close(); err != nil {
			s.logger.Warn("Error closing SSH connection", zap.Error(err))
		}
		s.rpcClient = nil
	}

	return nil
}

// ScrapeMetrics collects metrics
func (s *systemScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	s.collectionCount++
	now := pcommon.NewTimestampFromTime(time.Now())

	if s.rpcClient == nil {
		s.logger.Info("Establishing persistent SSH connection",
			zap.String("target", s.deviceTarget),
			zap.Int("collection_number", s.collectionCount))

		rpcClient, err := s.establishDeviceConnection(ctx, s.deviceTarget)
		if err != nil {
			// SSH connection failed - device is down
			s.logger.Error("Device connection failed - recording cisco.device.up=0",
				zap.String("target", s.deviceTarget),
				zap.Int("collection_number", s.collectionCount),
				zap.String("error_type", fmt.Sprintf("%T", err)),
				zap.Error(err))

			s.rpcClient = nil
			s.mb.RecordCiscoDeviceUpDataPoint(now, 0, s.deviceTarget)

			s.logger.Info("Device down - recorded metrics",
				zap.Float64("cisco.device.up", 0))

			return s.mb.Emit(), nil
		}

		s.rpcClient = rpcClient
		s.logger.Info("Persistent SSH connection established successfully",
			zap.String("target", s.deviceTarget),
			zap.String("os_type", s.rpcClient.GetOSType()))
	}

	s.mb.RecordCiscoDeviceUpDataPoint(now, 1, s.deviceTarget)

	if cpuUtil, err := s.collectCPUUtilization(ctx); err == nil {
		s.mb.RecordSystemCPUUtilizationDataPoint(now, cpuUtil, s.deviceTarget)
	} else {
		s.logger.Warn("Failed to collect CPU utilization, skipping metric",
			zap.Error(err))
	}

	if memUtil, err := s.collectMemoryUtilization(ctx); err == nil {
		s.mb.RecordSystemMemoryUtilizationDataPoint(now, memUtil, s.deviceTarget)
	} else {
		s.logger.Warn("Failed to collect memory utilization, skipping metric",
			zap.Error(err))
	}

	return s.mb.Emit(), nil
}

// establishDeviceConnection establishes SSH connection to Cisco device using shared connection factory
func (s *systemScraper) establishDeviceConnection(ctx context.Context, _ string) (*connection.RPCClient, error) {
	// Convert scraper device config to connection device config
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

	output, err := s.rpcClient.ExecuteCommand(command)
	if err != nil {
		return 0, fmt.Errorf("failed to execute memory command: %w", err)
	}

	// Parse memory utilization
	memUtil, err := parseMemoryUtilization(output, osType)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory utilization: %w", err)
	}

	return memUtil, nil
}
