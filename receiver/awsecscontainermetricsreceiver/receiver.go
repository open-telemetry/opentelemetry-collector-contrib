// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"
)

var _ receiver.Metrics = (*awsEcsContainerMetricsReceiver)(nil)

// awsEcsContainerMetricsReceiver implements the receiver.Metrics for aws ecs container metrics.
type awsEcsContainerMetricsReceiver struct {
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	config       *Config
	cancel       context.CancelFunc
	restClient   ecsutil.RestClient
	provider     *awsecscontainermetrics.StatsProvider
}

// New creates the aws ecs container metrics receiver with the given parameters.
func newAWSECSContainermetrics(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Metrics,
	rest ecsutil.RestClient) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	r := &awsEcsContainerMetricsReceiver{
		logger:       logger,
		nextConsumer: nextConsumer,
		config:       config,
		restClient:   rest,
	}
	return r, nil
}

// Start begins collecting metrics from Amazon ECS task metadata endpoint.
func (aecmr *awsEcsContainerMetricsReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, aecmr.cancel = context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(aecmr.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = aecmr.collectDataFromEndpoint(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// Shutdown stops the awsecscontainermetricsreceiver receiver.
func (aecmr *awsEcsContainerMetricsReceiver) Shutdown(context.Context) error {
	if aecmr.cancel != nil {
		aecmr.cancel()
	}
	return nil
}

// collectDataFromEndpoint collects container stats from Amazon ECS Task Metadata Endpoint
func (aecmr *awsEcsContainerMetricsReceiver) collectDataFromEndpoint(ctx context.Context) error {
	aecmr.provider = awsecscontainermetrics.NewStatsProvider(aecmr.restClient, aecmr.logger)
	stats, metadata, err := aecmr.provider.GetStats()

	if err != nil {
		aecmr.logger.Error("Failed to collect stats", zap.Error(err))
		return err
	}

	// TODO: report self metrics using obsreport
	mds := awsecscontainermetrics.MetricsData(stats, metadata, aecmr.logger)
	for _, md := range mds {
		err = aecmr.nextConsumer.ConsumeMetrics(ctx, md)
		if err != nil {
			return err
		}
	}

	return nil
}
