// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sclusterreceiver

import (
	"context"
	"fmt"
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	// Value of "type" key in configuration.
	typeStr = "k8s_cluster"

	// supported distributions
	distributionKubernetes = "kubernetes"
	distributionOpenShift  = "openshift"

	// Default config values.
	defaultCollectionInterval = 10 * time.Second
	defaultDistribution       = distributionKubernetes
)

var defaultNodeConditionsToReport = []string{"Ready"}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings:           config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Distribution:               defaultDistribution,
		CollectionInterval:         defaultCollectionInterval,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}
}

func createMetricsReceiver(
	_ context.Context, params component.ReceiverCreateSettings, cfg config.Receiver,
	consumer consumer.Metrics) (component.MetricsReceiver, error) {
	rCfg := cfg.(*Config)

	k8sClient, err := rCfg.getK8sClient()
	if err != nil {
		return nil, err
	}

	var osQuotaClient quotaclientset.Interface
	switch rCfg.Distribution {
	case distributionOpenShift:
		osQuotaClient, err = rCfg.getOpenShiftQuotaClient()
		if err != nil {
			return nil, err
		}
	case distributionKubernetes:
		// default case, nothing to initialize
	default:
		return nil, fmt.Errorf("\"%s\" is not a supported distribution. Must be one of: \"openshift\", \"kubernetes\"", rCfg.Distribution)
	}

	return newReceiver(params, rCfg, consumer, k8sClient, osQuotaClient)
}

// NewFactory creates a factory for k8s_cluster receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}
