// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ component.Config = (*Config)(nil)

// Config contains the information necessary for enabling the Datadog Extension.
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	// Define the site and API key (and whether to fail on invalid API key) in API.
	API datadogconfig.APIConfig `mapstructure:"api"`
	// If Hostname is empty extension will use available system APIs and cloud provider endpoints.
	Hostname string `mapstructure:"hostname"`
	// HTTPConfig is v2 config for the http metadata service.
	HTTPConfig *httpserver.Config `mapstructure:"http"`
	// DeploymentType indicates the type of deployment (gateway, daemonset, or unknown).
	// Defaults to "unknown" if not set.
	DeploymentType string `mapstructure:"deployment_type"`
	// InstallationMethod indicates how the collector was installed.
	// Valid values: "", "kubernetes", "bare-metal", "docker", "ecs-fargate", "eks-fargate".
	// Defaults to "" (unset) if not configured.
	InstallationMethod string `mapstructure:"installation_method"`
	// GatewayService is the k8s Service fronting the gateway collector pods.
	// Set by gateway collectors. Format: "service" or "namespace/service".
	// Together with cluster_name, it forms the join key for fleet topology queries.
	GatewayService string `mapstructure:"gateway_service"`
	// GatewayDestination is the k8s Service that this collector forwards telemetry to.
	// Set by agent/daemonset collectors. Format: "service" or "namespace/service".
	// Must match gateway_service on the receiving gateway collector.
	GatewayDestination string `mapstructure:"gateway_destination"`
}

// Validate ensures that the configuration is valid.
func (c *Config) Validate() error {
	if c.API.Site == "" {
		return datadogconfig.ErrEmptyEndpoint
	}
	if c.API.Key == "" {
		return datadogconfig.ErrUnsetAPIKey
	}
	if c.HTTPConfig == nil {
		return errors.New("http config is required")
	}
	// Validate deployment_type if set
	if c.DeploymentType != "" && c.DeploymentType != "gateway" && c.DeploymentType != "daemonset" && c.DeploymentType != "unknown" {
		return errors.New("deployment_type must be one of: gateway, daemonset, or unknown")
	}
	// Set default if not provided
	if c.DeploymentType == "" {
		c.DeploymentType = "unknown"
	}
	// Validate installation_method if set
	validInstallationMethods := []string{"", "kubernetes", "bare-metal", "docker", "ecs-fargate", "eks-fargate"}
	if !slices.Contains(validInstallationMethods, c.InstallationMethod) {
		return fmt.Errorf("installation_method must be one of: %s", strings.Join(validInstallationMethods[1:], ", "))
	}
	return nil
}
