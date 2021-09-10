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

package simpleprometheusreceiver

import (
	"context"
	"errors"
	"fmt"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

type prometheusReceiverWrapper struct {
	params            component.ReceiverCreateSettings
	config            *Config
	consumer          consumer.Metrics
	prometheusRecever component.MetricsReceiver
}

// new returns a prometheusReceiverWrapper
func new(params component.ReceiverCreateSettings, cfg *Config, consumer consumer.Metrics) *prometheusReceiverWrapper {
	return &prometheusReceiverWrapper{params: params, config: cfg, consumer: consumer}
}

// Start creates and starts the prometheus receiver.
func (prw *prometheusReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	pFactory := prometheusreceiver.NewFactory()

	pConfig, err := getPrometheusConfig(prw.config)
	if err != nil {
		return fmt.Errorf("failed to create prometheus receiver config: %v", err)
	}

	pr, err := pFactory.CreateMetricsReceiver(ctx, prw.params, pConfig, prw.consumer)
	if err != nil {
		return fmt.Errorf("failed to create prometheus receiver: %v", err)
	}

	prw.prometheusRecever = pr
	return prw.prometheusRecever.Start(ctx, host)
}

func getPrometheusConfig(cfg *Config) (*prometheusreceiver.Config, error) {
	var bearerToken string
	if cfg.UseServiceAccount {
		restConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		bearerToken = restConfig.BearerToken
		if bearerToken == "" {
			return nil, errors.New("bearer token was empty")
		}
	}

	out := &prometheusreceiver.Config{}
	httpConfig := configutil.HTTPClientConfig{}

	scheme := "http"

	if cfg.TLSEnabled {
		scheme = "https"
		httpConfig.TLSConfig = configutil.TLSConfig{
			CAFile:             cfg.TLSConfig.CAFile,
			CertFile:           cfg.TLSConfig.CertFile,
			KeyFile:            cfg.TLSConfig.KeyFile,
			InsecureSkipVerify: cfg.TLSConfig.InsecureSkipVerify,
		}
	}

	httpConfig.BearerToken = configutil.Secret(bearerToken)

	scrapeConfig := &config.ScrapeConfig{
		ScrapeInterval:  model.Duration(cfg.CollectionInterval),
		ScrapeTimeout:   model.Duration(cfg.CollectionInterval),
		JobName:         fmt.Sprintf("%s/%s", typeStr, cfg.Endpoint),
		HonorTimestamps: true,
		Scheme:          scheme,
		MetricsPath:     cfg.MetricsPath,
		Params:          cfg.Params,
		ServiceDiscoveryConfigs: discovery.Configs{
			&discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						{model.AddressLabel: model.LabelValue(cfg.Endpoint)},
					},
				},
			},
		},
	}

	scrapeConfig.HTTPClientConfig = httpConfig
	out.PrometheusConfig = &config.Config{ScrapeConfigs: []*config.ScrapeConfig{
		scrapeConfig,
	}}

	return out, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (prw *prometheusReceiverWrapper) Shutdown(ctx context.Context) error {
	return prw.prometheusRecever.Shutdown(ctx)
}
