// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// Factory for awscontainerinsightreceiver
const (
	// Key to invoke this receiver
	typeStr = "awscontainerinsightreceiver"

	// The stability of this receiver
	stability = component.StabilityLevelBeta

	// Default collection interval. Every 60s the receiver will collect metrics
	defaultCollectionInterval = 60 * time.Second

	// Default container orchestrator service is aws eks
	defaultContainerOrchestrator = "eks"

	// Metrics is tagged with service name by default
	defaultTagService = true

	// Don't use pod full name by default (as the full names contain suffix with random characters)
	defaultPrefFullPodName = false

	// Don't tag pod full name by default
	defaultAddFullPodNameMetricLabel = false

	// Don't tag container name by default
	defaultAddContainerNameMetricLabel = false

	// Rely on EC2 tags to auto-detect cluster name by default
	defaultClusterName = ""

	// Default locking resource name during EKS leader election
	defaultLeaderLockName = "otel-container-insight-clusterleader"

	// Don't enable EKS control plane metrics by default
	defaultEnableControlPlaneMetrics = false
)

// NewFactory creates a factory for AWS container insight receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

// createDefaultConfig returns a default config for the receiver.
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval:          defaultCollectionInterval,
		ContainerOrchestrator:       defaultContainerOrchestrator,
		TagService:                  defaultTagService,
		PrefFullPodName:             defaultPrefFullPodName,
		AddFullPodNameMetricLabel:   defaultAddFullPodNameMetricLabel,
		AddContainerNameMetricLabel: defaultAddContainerNameMetricLabel,
		ClusterName:                 defaultClusterName,
		LeaderLockName:              defaultLeaderLockName,
		EnableControlPlaneMetrics:   defaultEnableControlPlaneMetrics,
	}
}

// CreateMetricsReceiver creates an AWS Container Insight receiver.
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	rCfg := baseCfg.(*Config)
	return newAWSContainerInsightReceiver(params.TelemetrySettings, rCfg, consumer)
}
