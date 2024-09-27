// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server       OpAMPServer
	Agent        Agent
	Capabilities Capabilities `mapstructure:"capabilities"`
	Storage      Storage      `mapstructure:"storage"`
}

func (s Supervisor) Validate() error {
	if err := s.Server.Validate(); err != nil {
		return err
	}

	if err := s.Agent.Validate(); err != nil {
		return err
	}

	return nil
}

type Storage struct {
	// Directory is the directory where the Supervisor will store its data.
	Directory string `mapstructure:"directory"`
}

// Capabilities is the set of capabilities that the Supervisor supports.
type Capabilities struct {
	AcceptsRemoteConfig            bool `mapstructure:"accepts_remote_config"`
	AcceptsRestartCommand          bool `mapstructure:"accepts_restart_command"`
	AcceptsOpAMPConnectionSettings bool `mapstructure:"accepts_opamp_connection_settings"`
	AcceptsPackageAvailable        bool `mapstructure:"accepts_package_available"`
	ReportsEffectiveConfig         bool `mapstructure:"reports_effective_config"`
	ReportsOwnMetrics              bool `mapstructure:"reports_own_metrics"`
	ReportsHealth                  bool `mapstructure:"reports_health"`
	ReportsRemoteConfig            bool `mapstructure:"reports_remote_config"`
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

	if c.AcceptsPackageAvailable {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_AcceptsPackages
	}

	if c.ReportsPackageStatuses {
		supportedCapabilities |= protobufs.AgentCapabilities_AgentCapabilities_ReportsPackageStatuses
	}

	return supportedCapabilities
}

type OpAMPServer struct {
	Endpoint   string
	Headers    http.Header
	TLSSetting configtls.ClientConfig `mapstructure:"tls,omitempty"`
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

	err = o.TLSSetting.Validate()
	if err != nil {
		return fmt.Errorf("invalid server::tls settings: %w", err)
	}

	return nil
}

type Agent struct {
	Executable              string
	OrphanDetectionInterval time.Duration    `mapstructure:"orphan_detection_interval"`
	Description             AgentDescription `mapstructure:"description"`
	BootstrapTimeout        time.Duration    `mapstructure:"bootstrap_timeout"`
	HealthCheckPort         int              `mapstructure:"health_check_port"`
	Signature               AgentSignature   `mapstructure:"signature"`
}

func (a Agent) Validate() error {
	if a.OrphanDetectionInterval <= 0 {
		return errors.New("agent::orphan_detection_interval must be positive")
	}

	if a.BootstrapTimeout <= 0 {
		return errors.New("agent::bootstrap_timeout must be positive")
	}

	if a.HealthCheckPort < 0 || a.HealthCheckPort > 65535 {
		return errors.New("agent::health_check_port must be a valid port number")
	}

	if a.Executable == "" {
		return errors.New("agent::executable must be specified")
	}

	_, err := os.Stat(a.Executable)
	if err != nil {
		return fmt.Errorf("could not stat agent::executable path: %w", err)
	}

	if err := a.Signature.Validate(); err != nil {
		return err
	}

	return nil
}

type AgentSignature struct {
	// TODO: Certificate paths? The certificate can be specified via SIGSTORE_ROOT_FILE for now
	CertGithubWorkflowRepository string                   `mapstructure:"github_workflow_repository"`
	Identities                   []AgentSignatureIdentity `mapstructure:"identities"`
}

func (a AgentSignature) Validate() error {
	for i, ident := range a.Identities {
		if err := ident.Validate(); err != nil {
			return fmt.Errorf("agent::identities[%d]: %w", i, err)
		}
	}

	return nil
}

type AgentSignatureIdentity struct {
	Issuer        string `mapstructure:"issuer"`
	Subject       string `mapstructure:"subject"`
	IssuerRegExp  string `mapstructure:"issuer_regex"`
	SubjectRegExp string `mapstructure:"subject_regex"`
}

func (a AgentSignatureIdentity) Validate() error {
	if a.Issuer != "" && a.IssuerRegExp != "" {
		return errors.New("cannot specify both issuer and issuer_regex")
	}

	if a.Subject != "" && a.SubjectRegExp != "" {
		return errors.New("cannot specify both subject and subject_regex")
	}

	if a.Issuer == "" && a.IssuerRegExp == "" {
		return errors.New("must specify one of issuer or issuer_regex")
	}

	if a.Subject == "" && a.SubjectRegExp == "" {
		return errors.New("must specify one of subject or subject_regex")
	}

	return nil
}

type AgentDescription struct {
	IdentifyingAttributes    map[string]string `mapstructure:"identifying_attributes"`
	NonIdentifyingAttributes map[string]string `mapstructure:"non_identifying_attributes"`
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
			AcceptsPackageAvailable:        false,
			ReportsEffectiveConfig:         true,
			ReportsOwnMetrics:              true,
			ReportsHealth:                  true,
			ReportsRemoteConfig:            false,
			ReportsPackageStatuses:         false,
		},
		Storage: Storage{
			Directory: defaultStorageDir,
		},
		Agent: Agent{
			OrphanDetectionInterval: 5 * time.Second,
			BootstrapTimeout:        3 * time.Second,
			Signature: AgentSignature{
				CertGithubWorkflowRepository: "open-telemetry/opentelemetry-collector-releases",
				Identities: []AgentSignatureIdentity{
					{
						Issuer:        "https://token.actions.githubusercontent.com",
						SubjectRegExp: `^https://github.com/open-telemetry/opentelemetry-collector-releases/.github/workflows/base-release.yaml@refs/tags/[^/]*$`,
					},
				},
			},
		},
	}
}
