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

package kubeletstatsreceiver

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

const (
	typeStr            = "kubeletstats"
	metricGroupsConfig = "metric_groups"
)

var defaultMetricGroups = []kubelet.MetricGroup{
	kubelet.ContainerMetricGroup,
	kubelet.PodMetricGroup,
	kubelet.NodeMetricGroup,
}

var _ component.ReceiverFactoryBase = (*Factory)(nil)

type Factory struct {
}

func (f *Factory) Type() configmodels.Type {
	return typeStr
}

func (f *Factory) CreateDefaultConfig() configmodels.Receiver {
	return &Config{
		ReceiverSettings: configmodels.ReceiverSettings{
			TypeVal: typeStr,
		},
		ClientConfig: kubelet.ClientConfig{
			APIConfig: k8sconfig.APIConfig{
				AuthType: k8sconfig.AuthTypeTLS,
			},
		},
		CollectionInterval: 10 * time.Second,
	}
}

func (f *Factory) CustomUnmarshaler() component.CustomUnmarshaler {
	return customUnmarshaller
}

func customUnmarshaller(sourceViperSection *viper.Viper, intoCfg interface{}) error {
	if sourceViperSection == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := sourceViperSection.Unmarshal(intoCfg); err != nil {
		return err
	}

	config := intoCfg.(*Config)

	// custom unmarhalling is required to get []kubelet.MetricGroup, the default
	// unmarshaller only supports string slices.
	if !sourceViperSection.IsSet(metricGroupsConfig) {
		config.MetricGroupsToCollect = defaultMetricGroups
		return nil
	}
	mgs := sourceViperSection.Get(metricGroupsConfig)

	out, err := yaml.Marshal(mgs)
	if err != nil {
		return fmt.Errorf("failed to marshal %s to yaml: %w", metricGroupsConfig, err)
	}

	err = yaml.UnmarshalStrict(out, &config.MetricGroupsToCollect)
	if err != nil {
		return fmt.Errorf("failed to retrieve %s: %w", metricGroupsConfig, err)
	}

	return nil
}

func (f *Factory) CreateTraceReceiver(
	context.Context,
	*zap.Logger,
	configmodels.Receiver,
	consumer.TraceConsumerOld,
) (component.TraceReceiver, error) {
	return nil, configerror.ErrDataTypeIsNotSupported
}

func (f *Factory) CreateMetricsReceiver(
	ctx context.Context,
	logger *zap.Logger,
	baseCfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	cfg := baseCfg.(*Config)
	rOptions, err := cfg.getReceiverOptions()
	if err != nil {
		return nil, err
	}
	rest, err := f.restClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	return newReceiver(rOptions, logger, rest, consumer)
}

func (f *Factory) restClient(logger *zap.Logger, cfg *Config) (kubelet.RestClient, error) {
	clientProvider, err := kubelet.NewClientProvider(cfg.Endpoint, &cfg.ClientConfig, logger)
	if err != nil {
		return nil, err
	}
	client, err := clientProvider.BuildClient()
	if err != nil {
		return nil, err
	}
	rest := kubelet.NewRestClient(client)
	return rest, nil
}
