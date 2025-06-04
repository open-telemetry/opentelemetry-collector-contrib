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
	params             receiver.Settings
	config             *Config
	consumer           consumer.Metrics
	prometheusReceiver receiver.Metrics
}

// newPrometheusReceiverWrapper returns a prometheusReceiverWrapper
func newPrometheusReceiverWrapper(params receiver.Settings, cfg *Config, consumer consumer.Metrics) *prometheusReceiverWrapper {
	return &prometheusReceiverWrapper{params: params, config: cfg, consumer: consumer}
}

// Start creates and starts the prometheus receiver.
func (prw *prometheusReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	pFactory := prometheusreceiver.NewFactory()

	pConfig, err := getPrometheusConfigWrapper(prw.config, prw.params)
	if err != nil {
		return fmt.Errorf("failed to create prometheus receiver config: %w", err)
	}

	params := receiver.Settings{
		ID:                component.NewIDWithName(pFactory.Type(), prw.params.ID.String()),
		TelemetrySettings: prw.params.TelemetrySettings,
		BuildInfo:         prw.params.BuildInfo,
	}
	pr, err := pFactory.CreateMetrics(ctx, params, pConfig, prw.consumer)
	if err != nil {
		return fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	prw.prometheusReceiver = pr
	return prw.prometheusReceiver.Start(ctx, host)
}

// Deprecated: [v0.55.0] Use getPrometheusConfig instead.
func getPrometheusConfigWrapper(cfg *Config, params receiver.Settings) (*prometheusreceiver.Config, error) {
	if cfg.TLSEnabled {
		params.Logger.Warn("the `tls_config` and 'tls_enabled' settings are deprecated, please use `tls` instead")
		cfg.TLS = configtls.ClientConfig{
			Config: configtls.Config{
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

	tlsConfig, err := cfg.TLS.LoadTLSConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("tls config is not valid: %w", err)
	}
	if tlsConfig != nil {
		scheme = "https"
		httpConfig.TLSConfig = configutil.TLSConfig{
			CAFile:             cfg.TLS.CAFile,
			CertFile:           cfg.TLS.CertFile,
			KeyFile:            cfg.TLS.KeyFile,
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}
	}

	httpConfig.BearerToken = configutil.Secret(bearerToken)

	labels := make(model.LabelSet, len(cfg.Labels)+1)
	for k, v := range cfg.Labels {
		labels[model.LabelName(k)] = model.LabelValue(v)
	}
	labels[model.AddressLabel] = model.LabelValue(cfg.Endpoint)

	jobName := cfg.JobName
	if jobName == "" {
		jobName = fmt.Sprintf("%s/%s", metadata.Type, cfg.Endpoint)
	}
	scrapeConfig := &config.ScrapeConfig{
		ScrapeInterval:  model.Duration(cfg.CollectionInterval),
		ScrapeTimeout:   model.Duration(cfg.CollectionInterval),
		JobName:         jobName,
		HonorTimestamps: true,
		Scheme:          scheme,
		MetricsPath:     cfg.MetricsPath,
		Params:          cfg.Params,
		ServiceDiscoveryConfigs: discovery.Configs{
			discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						labels,
					},
				},
			},
		},
	}

	scrapeConfig.HTTPClientConfig = httpConfig
	out.PrometheusConfig = &prometheusreceiver.PromConfig{
		GlobalConfig: config.DefaultGlobalConfig,
		ScrapeConfigs: []*config.ScrapeConfig{
			scrapeConfig,
		},
	}

	return out, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (prw *prometheusReceiverWrapper) Shutdown(ctx context.Context) error {
	if prw.prometheusReceiver == nil {
		return nil
	}
	return prw.prometheusReceiver.Shutdown(ctx)
}
