// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

const (
	// supported distributions
	distributionKubernetes = "kubernetes"
	distributionOpenShift  = "openshift"

	// Default config values.
	defaultCollectionInterval = 10 * time.Second
	defaultDistribution       = distributionKubernetes
)

var defaultNodeConditionsToReport = []string{"Ready"}

func createDefaultConfig() component.Config {
	return &Config{
		Distribution:               defaultDistribution,
		CollectionInterval:         defaultCollectionInterval,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}
}

// NewFactory creates a factory for k8s_cluster receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(newReceiver, metadata.MetricsStability))
}
