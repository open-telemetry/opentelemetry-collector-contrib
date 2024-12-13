// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightskueuereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	kueuescraper "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver/internal/kueuescraper"
)

var _ receiver.Metrics = (*awsContainerInsightKueueReceiver)(nil)

// awsContainerInsightKueueReceiver implements the receiver.Metrics
type awsContainerInsightKueueReceiver struct {
	settings     component.TelemetrySettings
	nextConsumer consumer.Metrics
	config       *Config
	cancel       context.CancelFunc
	kueueScraper *kueuescraper.KueuePrometheusScraper
}

// newAWSContainerInsightReceiver creates the aws container insight receiver with the given parameters.
func newAWSContainerInsightReceiver(
	settings component.TelemetrySettings,
	config *Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	r := &awsContainerInsightKueueReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       config,
	}
	return r, nil
}

// Start collecting metrics from cadvisor and k8s api server (if it is an elected leader)
func (akr *awsContainerInsightKueueReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, akr.cancel = context.WithCancel(ctx)

	// wait for kubelet availability, but don't block on it
	go func() {
		if err := akr.init(ctx, host); err != nil {
			akr.settings.Logger.Error("Unable to initialize receiver for Kueue metrics", zap.Error(err))
			return
		}
		akr.start(ctx)
	}()

	return nil
}

func (akr *awsContainerInsightKueueReceiver) init(ctx context.Context, host component.Host) error {
	if runtime.GOOS == ci.OperatingSystemWindows {
		return fmt.Errorf("unsupported operating system: %s", ci.OperatingSystemWindows)
	}

	err := akr.initKueuePrometheusScraper(ctx, host)
	if err != nil {
		akr.settings.Logger.Warn("Unable to start kueue prometheus scraper", zap.Error(err))
		return err
	}
	return nil
}

func (akr *awsContainerInsightKueueReceiver) start(ctx context.Context) {
	ticker := time.NewTicker(akr.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			akr.collectData()
		case <-ctx.Done():
			return
		}
	}
}

func (akr *awsContainerInsightKueueReceiver) initKueuePrometheusScraper(
	ctx context.Context,
	host component.Host,
) error {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	bearerToken := restConfig.BearerToken
	if bearerToken == "" {
		return errors.New("bearer token was empty")
	}

	akr.kueueScraper, err = kueuescraper.NewKueuePrometheusScraper(kueuescraper.KueuePrometheusScraperOpts{
		Ctx:               ctx,
		TelemetrySettings: akr.settings,
		Consumer:          akr.nextConsumer,
		Host:              host,
		ClusterName:       akr.config.ClusterName,
		BearerToken:       bearerToken,
	})
	return err
}

// Shutdown stops the awsContainerInsightKueueReceiver receiver.
func (akr *awsContainerInsightKueueReceiver) Shutdown(context.Context) error {
	if akr.cancel == nil {
		return nil
	}
	akr.cancel()

	if akr.kueueScraper != nil {
		akr.kueueScraper.Shutdown()
	}

	return nil
}

func (akr *awsContainerInsightKueueReceiver) collectData() {
	if akr.kueueScraper != nil {
		// this does not return any metrics, it just ensures scraping is running on elected leader node
		akr.kueueScraper.GetMetrics() //nolint:errcheck
	}
}
