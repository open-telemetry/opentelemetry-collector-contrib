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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
)

const typeStr = "kubeletstats"

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
	cfg configmodels.Receiver,
	consumer consumer.MetricsConsumerOld,
) (component.MetricsReceiver, error) {
	rest, err := f.restClient(logger, cfg)
	if err != nil {
		return nil, err
	}
	return &receiver{
		logger:   logger,
		cfg:      cfg,
		consumer: consumer,
		rest:     rest,
	}, nil
}

func (f *Factory) restClient(logger *zap.Logger, baseCfg configmodels.Receiver) (kubelet.RestClient, error) {
	cfg := baseCfg.(*Config)
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
