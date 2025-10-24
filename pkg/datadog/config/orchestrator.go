// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
import (
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
)

// OrchestratorConfig defines orchestrator exporter specific configuration
type OrchestratorConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog orchestrator intake server to send data to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `mapstructure:",squash"`

	// ClusterName is the name of the Kubernetes cluster to associate with the orchestrator data.
	ClusterName string `mapstructure:"cluster_name"`

	// Endpoints is a map of orchestrator endpoints to send data to.
	// The key is the endpoint URL, and the value is the API key to use for that endpoint.
	Endpoints []OrchestratorEndpoint `mapstructure:"endpoints"`
}

// OrchestratorEndpoint defines an orchestrator endpoint configuration
type OrchestratorEndpoint struct {
	// Endpoint is the URL of the orchestrator endpoint to send data to.
	Endpoint string `mapstructure:"endpoint"`
	// Key is the API key to use for the orchestrator endpoint.
	Key configopaque.String `mapstructure:"key"`
}
