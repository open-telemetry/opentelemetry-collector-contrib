// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

// NewFactory creates a factory for Cisco OS receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Devices:          []DeviceConfig{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	conf := cfg.(*Config)

	scraper := newConnectionScraper(conf.Devices, set.Logger)

	return scraperhelper.NewMetricsController(
		&conf.ControllerConfig,
		set,
		consumer,
		scraperhelper.AddScraper(metadata.Type, scraper),
	)
}

// connectionScraper implements basic SSH connection verification and connection status metrics
type connectionScraper struct {
	devices []DeviceConfig
	logger  *zap.Logger
	mb      *metadata.MetricsBuilder
}

func newConnectionScraper(devices []DeviceConfig, logger *zap.Logger) *connectionScraper {
	return &connectionScraper{
		devices: devices,
		logger:  logger,
		mb: metadata.NewMetricsBuilder(
			metadata.DefaultMetricsBuilderConfig(),
			receiver.Settings{
				ID:                component.MustNewIDWithName(metadata.Type.String(), "connection"),
				TelemetrySettings: component.TelemetrySettings{Logger: logger},
			},
		),
	}
}

func (*connectionScraper) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*connectionScraper) Shutdown(_ context.Context) error {
	return nil
}

func (s *connectionScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	s.mb.Reset(metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))

	for _, device := range s.devices {
		// Test SSH connection
		connectionStatus := int64(0)

		if client, err := s.establishConnection(ctx, device); err != nil {
			s.logger.Warn("Failed to connect to device", zap.String("device", device.Device.Host.IP), zap.Error(err))
		} else {
			connectionStatus = int64(1)
			client.Close()
		}

		// Create resource attributes
		rb := s.mb.NewResourceBuilder()
		rb.SetCiscoDeviceIP(device.Device.Host.IP)
		if device.Device.Host.Name != "" {
			rb.SetCiscoDeviceName(device.Device.Host.Name)
		}

		// Record connection status metric
		now := pcommon.NewTimestampFromTime(time.Now())
		s.mb.RecordCiscoConnectionStatusDataPoint(now, connectionStatus)
	}

	return s.mb.Emit(), nil
}

// establishConnection creates a new SSH connection to test connectivity
func (s *connectionScraper) establishConnection(_ context.Context, device DeviceConfig) (*connection.Client, error) {
	// Build SSH config
	sshConfig := &cryptossh.ClientConfig{
		User:            device.Auth.Username,
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(), // nolint: gosec // G106: acceptable for alpha
		Timeout:         10 * time.Second,
	}

	// Add authentication method
	if device.Auth.Password != "" {
		sshConfig.Auth = []cryptossh.AuthMethod{
			cryptossh.Password(string(device.Auth.Password)),
		}
	} else if device.Auth.KeyFile != "" {
		key, err := os.ReadFile(device.Auth.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %w", err)
		}

		signer, err := cryptossh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %w", err)
		}

		sshConfig.Auth = []cryptossh.AuthMethod{
			cryptossh.PublicKeys(signer),
		}
	}

	// Connect to device
	address := fmt.Sprintf("%s:%d", device.Device.Host.IP, device.Device.Host.Port)
	conn, err := cryptossh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed to %s: %w", address, err)
	}

	// Create client wrapper
	client := &connection.Client{
		Target:     address,
		Username:   device.Auth.Username,
		Connection: conn,
		Logger:     s.logger,
	}

	return client, nil
}
