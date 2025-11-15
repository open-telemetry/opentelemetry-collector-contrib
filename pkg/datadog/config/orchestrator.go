// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
import (
	"go.opentelemetry.io/collector/config/confignet"
)

// OrchestratorConfig defines orchestrator exporter specific configuration
type OrchestratorConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog orchestrator intake server to send data to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `mapstructure:",squash"`

	// ClusterName is the name of the Kubernetes cluster to associate with the orchestrator data.
	ClusterName string `mapstructure:"cluster_name"`
	
	Enabled bool `mapstructure:"enabled"`
}
