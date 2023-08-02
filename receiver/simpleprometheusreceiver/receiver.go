// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"

import (
	"context"
	"errors"
	"fmt"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

type prometheusReceiverWrapper struct {
	params            receiver.CreateSettings
	config            *Config
	consumer          consumer.Metrics
	prometheusRecever receiver.Metrics
}

// newPrometheusReceiverWrapper returns a prometheusReceiverWrapper
func newPrometheusReceiverWrapper(params receiver.CreateSettings, cfg *Config, consumer consumer.Metrics) *prometheusReceiverWrapper {
	return &prometheusReceiverWrapper{params: params, config: cfg, consumer: consumer}
}

// Start creates and starts the prometheus receiver.
func (prw *prometheusReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	pFactory := prometheusreceiver.NewFactory()

	pConfig, err := getPrometheusConfigWrapper(prw.config, prw.params)
	if err != nil {
		return fmt.Errorf("failed to create prometheus receiver config: %w", err)
	}

	pr, err := pFactory.CreateMetricsReceiver(ctx, prw.params, pConfig, prw.consumer)
	if err != nil {
		return fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	prw.prometheusRecever = pr
	return prw.prometheusRecever.Start(ctx, host)
}

// Deprecated: [v0.55.0] Use getPrometheusConfig instead.
func getPrometheusConfigWrapper(cfg *Config, params receiver.CreateSettings) (*prometheusreceiver.Config, error) {
	if cfg.TLSEnabled {
		params.Logger.Warn("the `tls_config` and 'tls_enabled' settings are deprecated, please use `tls` instead")
		cfg.HTTPClientSettings.TLSSetting = configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile:   cfg.TLSConfig.CAFile,
				CertFile: cfg.TLSConfig.CertFile,
				KeyFile:  cfg.TLSConfig.KeyFile,
			},
			Insecure:           false,
			InsecureSkipVerify: cfg.TLSConfig.InsecureSkipVerify,
		}
	}
	return getPrometheusConfig(cfg)
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

	tlsConfig, err := cfg.TLSSetting.LoadTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("tls config is not valid: %w", err)
	}
	if tlsConfig != nil {
		scheme = "https"
		httpConfig.TLSConfig = configutil.TLSConfig{
			CAFile:             cfg.TLSSetting.CAFile,
			CertFile:           cfg.TLSSetting.CertFile,
			KeyFile:            cfg.TLSSetting.KeyFile,
			InsecureSkipVerify: cfg.TLSSetting.InsecureSkipVerify,
		}
	}

	httpConfig.BearerToken = configutil.Secret(bearerToken)

	labels := make(model.LabelSet, len(cfg.Labels)+1)
	for k, v := range cfg.Labels {
		labels[model.LabelName(k)] = model.LabelValue(v)
	}
	labels[model.AddressLabel] = model.LabelValue(cfg.Endpoint)

	scrapeConfig := &config.ScrapeConfig{
		ScrapeInterval:  model.Duration(cfg.CollectionInterval),
		ScrapeTimeout:   model.Duration(cfg.CollectionInterval),
		JobName:         fmt.Sprintf("%s/%s", metadata.Type, cfg.Endpoint),
		HonorTimestamps: true,
		Scheme:          scheme,
		MetricsPath:     cfg.MetricsPath,
		Params:          cfg.Params,
		ServiceDiscoveryConfigs: discovery.Configs{
			&discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						labels,
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
	if prw.prometheusRecever == nil {
		return nil
	}
	return prw.prometheusRecever.Shutdown(ctx)
}
