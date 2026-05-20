// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	config "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"
)

var _ confmap.Validator = (*Supervisor)(nil)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       OpAMPServer       `mapstructure:"server"`
	Agent        Agent             `mapstructure:"agent"`
	Capabilities Capabilities      `mapstructure:"capabilities"`
	Storage      Storage           `mapstructure:"storage"`
	Telemetry    Telemetry         `mapstructure:"telemetry"`
	HealthCheck  HealthCheck       `mapstructure:"healthcheck"`
	Extensions   extensions.Config `mapstructure:"extensions,omitempty"`
}

// Load loads the Supervisor config from a file.
func Load(configFile string) (Supervisor, error) {
	if configFile == "" {
		return Supervisor{}, errors.New("path to config file cannot be empty")
	}

	resolverSettings := confmap.ResolverSettings{
		URIs: []string{configFile},
		ProviderFactories: []confmap.ProviderFactory{
			fileprovider.NewFactory(),
			envprovider.NewFactory(),
		},
		ConverterFactories: []confmap.ConverterFactory{},
		DefaultScheme:      "env",
	}

	resolver, err := confmap.NewResolver(resolverSettings)
	if err != nil {
		return Supervisor{}, err
	}

	conf, err := resolver.Resolve(context.Background())
	if err != nil {
		return Supervisor{}, err
	}

	cfg := DefaultSupervisor()
	if err := conf.Unmarshal(&cfg); err != nil {
		return Supervisor{}, err
	}

	return cfg, nil
}

func (Supervisor) Validate() error {
	return nil
}

type Storage struct {
	// Directory is the directory where the Supervisor will store its data.
	Directory string `mapstructure:"directory"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Capabilities is the set of capabilities that the Supervisor supports.
type Capabilities struct {
	AcceptsRemoteConfig            bool `mapstructure:"accepts_remote_config"`
	AcceptsRestartCommand          bool `mapstructure:"accepts_restart_command"`
	AcceptsOpAMPConnectionSettings bool `mapstructure:"accepts_opamp_connection_settings"`
	ReportsEffectiveConfig         bool `mapstructure:"reports_effective_config"`
	ReportsOwnMetrics              bool `mapstructure:"reports_own_metrics"`
	ReportsOwnLogs                 bool `mapstructure:"reports_own_logs"`
	ReportsOwnTraces               bool `mapstructure:"reports_own_traces"`
	ReportsHealth                  bool `mapstructure:"reports_health"`
	ReportsRemoteConfig            bool `mapstructure:"reports_remote_config"`
	ReportsAvailableComponents     bool `mapstructure:"reports_available_components"`
	ReportsHeartbeat               bool `mapstructure:"reports_heartbeat"`
	AcceptsPackages                bool `mapstructure:"accepts_packages"`
	ReportsPackageStatuses         bool `mapstructure:"reports_package_statuses"`
}

func (c Capabilities) SupportedCapabilities() protobufs.AgentCapabilities {
	supportedCapabilities := protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus

	if c.ReportsEffectiveConfig {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig
	}

	if c.ReportsHealth {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth
	}

	if c.ReportsOwnMetrics {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics
	}

	if c.ReportsOwnLogs {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnLogs
	}

	if c.ReportsOwnTraces {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnTraces
	}

	if c.AcceptsRemoteConfig {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig
	}

	if c.ReportsRemoteConfig {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig
	}

	if c.AcceptsRestartCommand {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsRestartCommand
	}

	if c.AcceptsOpAMPConnectionSettings {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsOpAMPConnectionSettings
	}

	if c.ReportsAvailableComponents {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents
	}
	if c.ReportsHeartbeat {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat
	}

	// AcceptsPackages and ReportsPackageStatuses are not yet fully implemented.
	// They are included here for completeness.
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/47272
	if c.AcceptsPackages {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages
	}
	if c.ReportsPackageStatuses {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses
	}

	return supportedCapabilities
}

type OpAMPServer struct {
	Endpoint string                 `mapstructure:"endpoint"`
	Headers  http.Header            `mapstructure:"headers"`
	TLS      configtls.ClientConfig `mapstructure:"tls,omitempty"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (o OpAMPServer) OpaqueHeaders() map[string][]configopaque.String {
	opaque := make(map[string][]configopaque.String, len(o.Headers))

	for key, values := range o.Headers {
		opaque[key] = make([]configopaque.String, len(values))
		for i, val := range values {
			opaque[key][i] = configopaque.String(val)
		}
	}

	return opaque
}

func (o OpAMPServer) Validate() error {
	if o.Endpoint == "" {
		return errors.New("server::endpoint must be specified")
	}

	url, err := url.Parse(o.Endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL for server::endpoint: %w", err)
	}

	switch url.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf(`invalid scheme %q for server::endpoint, must be one of "http", "https", "ws", or "wss"`, url.Scheme)
	}

	err = o.TLS.Validate()
	if err != nil {
		return fmt.Errorf("invalid server::tls settings: %w", err)
	}

	return nil
}

type Agent struct {
	Executable              string            `mapstructure:"executable"`
	InstanceID              string            `mapstructure:"instance_id"`
	OrphanDetectionInterval time.Duration     `mapstructure:"orphan_detection_interval"`
	Description             AgentDescription  `mapstructure:"description"`
	ConfigApplyTimeout      time.Duration     `mapstructure:"config_apply_timeout"`
	BootstrapTimeout        time.Duration     `mapstructure:"bootstrap_timeout"`
	OpAMPServerPort         int               `mapstructure:"opamp_server_port"`
	PassthroughLogs         bool              `mapstructure:"passthrough_logs"`
	UseHUPConfigReload      bool              `mapstructure:"use_hup_config_reload"`
	ValidateConfig          bool              `mapstructure:"validate_config"`
	ConfigFiles             []string          `mapstructure:"config_files"`
	Arguments               []string          `mapstructure:"args"`
	Env                     map[string]string `mapstructure:"env"`
}

func (a Agent) Validate() error {
	if a.OrphanDetectionInterval <= 0 {
		return errors.New("agent::orphan_detection_interval must be positive")
	}

	if a.BootstrapTimeout <= 0 {
		return errors.New("agent::bootstrap_timeout must be positive")
	}

	if a.OpAMPServerPort < 0 || a.OpAMPServerPort > 65535 {
		return errors.New("agent::opamp_server_port must be a valid port number")
	}

	if a.InstanceID != "" {
		if _, err := uuid.Parse(a.InstanceID); err != nil {
			return fmt.Errorf("agent::instance_id must be a valid UUID string when set: %w", err)
		}
	}

	if a.Executable == "" {
		return errors.New("agent::executable must be specified")
	}

	_, err := os.Stat(a.Executable)
	if err != nil {
		return fmt.Errorf("could not stat agent::executable path: %w", err)
	}

	if a.ConfigApplyTimeout <= 0 {
		return errors.New("agent::config_apply_timeout must be valid duration")
	}

	for _, file := range a.ConfigFiles {
		if !strings.HasPrefix(file, "$") {
			continue
		}
		if !slices.Contains(SpecialConfigFiles, SpecialConfigFile(file)) {
			return fmt.Errorf("agent::config_files contains invalid special file: %q. Must be one of %v", file, SpecialConfigFiles)
		}
	}

	if runtime.GOOS == "windows" && a.UseHUPConfigReload {
		return errors.New("agent::use_hup_config_reload is not supported on Windows")
	}

	return nil
}

type SpecialConfigFile string

const (
	SpecialConfigFileOwnTelemetry   SpecialConfigFile = "$OWN_TELEMETRY_CONFIG"
	SpecialConfigFileOpAMPExtension SpecialConfigFile = "$OPAMP_EXTENSION_CONFIG"
	SpecialConfigFileRemoteConfig   SpecialConfigFile = "$REMOTE_CONFIG"
)

var SpecialConfigFiles = []SpecialConfigFile{
	SpecialConfigFileOwnTelemetry,
	SpecialConfigFileOpAMPExtension,
	SpecialConfigFileRemoteConfig,
}

type AgentDescription struct {
	IdentifyingAttributes    map[string]string `mapstructure:"identifying_attributes"`
	NonIdentifyingAttributes map[string]string `mapstructure:"non_identifying_attributes"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Telemetry struct {
	// TODO: Add more telemetry options
	// Issue here: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35582
	Logs    Logs                           `mapstructure:"logs"`
	Metrics Metrics                        `mapstructure:"metrics"`
	Traces  otelconftelemetry.TracesConfig `mapstructure:"traces"`

	Resource otelconftelemetry.ResourceConfig `mapstructure:"resource"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type ResourceConfig = otelconftelemetry.ResourceConfig

type HealthCheck struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (h HealthCheck) Port() int64 {
	_, port, err := net.SplitHostPort(h.NetAddr.Endpoint)
	if err != nil {
		return 0
	}
	parsedPort, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		return 0
	}
	return parsedPort
}

func (h HealthCheck) Validate() error {
	parsedPort := h.Port()
	if parsedPort < 0 || parsedPort > 65535 {
		return fmt.Errorf("healthcheck::endpoint must contain a valid port number, got %d", parsedPort)
	}
	return nil
}

type Logs struct {
	Level            zapcore.Level `mapstructure:"level"`
	ErrorOutputPaths []string      `mapstructure:"error_output_paths"`
	OutputPaths      []string      `mapstructure:"output_paths"`
	// Processors allow configuration of log record processors to emit logs to
	// any number of supported backends.
	Processors []config.LogRecordProcessor `mapstructure:"processors,omitempty"`
}

type Metrics struct {
	Level   configtelemetry.Level `mapstructure:"level"`
	Readers []config.MetricReader `mapstructure:"readers"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// DefaultSupervisor returns the default supervisor config
func DefaultSupervisor() Supervisor {
	defaultStorageDir := "/var/lib/otelcol/supervisor"
	if runtime.GOOS == "windows" {
		// Windows default is "%ProgramData%\Otelcol\Supervisor"
		// If the ProgramData environment variable is not set,
		// it falls back to C:\ProgramData
		programDataDir := os.Getenv("ProgramData")
		if programDataDir == "" {
			programDataDir = `C:\ProgramData`
		}

		defaultStorageDir = filepath.Join(programDataDir, "Otelcol", "Supervisor")
	}

	return Supervisor{
		Capabilities: Capabilities{
			AcceptsRemoteConfig:            false,
			AcceptsRestartCommand:          false,
			AcceptsOpAMPConnectionSettings: false,
			ReportsEffectiveConfig:         true,
			ReportsOwnMetrics:              true,
			ReportsOwnLogs:                 false,
			ReportsOwnTraces:               false,
			ReportsHealth:                  true,
			ReportsRemoteConfig:            false,
			ReportsAvailableComponents:     false,
			ReportsHeartbeat:               true,
			AcceptsPackages:                false,
			ReportsPackageStatuses:         false,
		},
		Storage: Storage{
			Directory: defaultStorageDir,
		},
		Agent: Agent{
			OrphanDetectionInterval: 5 * time.Second,
			ConfigApplyTimeout:      5 * time.Second,
			BootstrapTimeout:        3 * time.Second,
			PassthroughLogs:         false,
			ValidateConfig:          false,
		},
		Telemetry: Telemetry{
			Logs: Logs{
				Level:            zapcore.InfoLevel,
				OutputPaths:      []string{"stdout"},
				ErrorOutputPaths: []string{"stderr"},
			},
		},
		HealthCheck: HealthCheck{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: confignet.TransportTypeTCP,
				},
			},
		},
	}
}
