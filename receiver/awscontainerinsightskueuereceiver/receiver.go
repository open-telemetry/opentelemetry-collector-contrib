// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightskueuereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver"

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver/internal/kueuescraper"
)

var _ receiver.Metrics = (*awsContainerInsightsKueueReceiver)(nil)

// awsContainerInsightsKueueReceiver implements receiver.Metrics
type awsContainerInsightsKueueReceiver struct {
	settings     component.TelemetrySettings
	nextConsumer consumer.Metrics
	config       *Config
	cancel       context.CancelFunc
	kueueScraper *kueuescraper.KueuePrometheusScraper
}

// newAWSContainerInsightsKueueReceiver creates the aws container insights kueue receiver with the given parameters.
func newAWSContainerInsightsKueueReceiver(
	settings component.TelemetrySettings,
	config *Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	r := &awsContainerInsightsKueueReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       config,
	}
	return r, nil
}

// Start collecting metrics from kueue metrics scraper
func (akr *awsContainerInsightsKueueReceiver) Start(ctx context.Context, host component.Host) error {
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

func (akr *awsContainerInsightsKueueReceiver) init(ctx context.Context, host component.Host) error {
	if runtime.GOOS == ci.OperatingSystemWindows {
		return fmt.Errorf("unsupported operating system: %s", ci.OperatingSystemWindows)
	}

	err := akr.initKueuePrometheusScraper(ctx, host)
	if err != nil {
		akr.settings.Logger.Warn("Unable to start Prometheus scraper for Kueue metrics", zap.Error(err))
		return err
	}
	return nil
}

func (akr *awsContainerInsightsKueueReceiver) start(ctx context.Context) {
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

func (akr *awsContainerInsightsKueueReceiver) initKueuePrometheusScraper(
	ctx context.Context,
	host component.Host,
) error {
	var err error
	akr.kueueScraper, err = kueuescraper.NewKueuePrometheusScraper(kueuescraper.KueuePrometheusScraperOpts{
		Ctx:               ctx,
		TelemetrySettings: akr.settings,
		Consumer:          akr.nextConsumer,
		Host:              host,
		ClusterName:       akr.config.ClusterName,
	})
	return err
}

// Shutdown stops the awsContainerInsightKueueReceiver receiver.
func (akr *awsContainerInsightsKueueReceiver) Shutdown(context.Context) error {
	if akr.cancel == nil {
		return nil
	}
	akr.cancel()

	if akr.kueueScraper != nil {
		akr.kueueScraper.Shutdown()
	}

	return nil
}

func (akr *awsContainerInsightsKueueReceiver) collectData() {
	if akr.kueueScraper != nil {
		// this does not return any metrics, it just ensures scraping is running
		akr.kueueScraper.GetMetrics() //nolint:errcheck
	}
}
