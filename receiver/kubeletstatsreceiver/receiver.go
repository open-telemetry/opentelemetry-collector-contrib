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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
)

var _ component.MetricsReceiver = (*receiver)(nil)

type receiver struct {
	options  *receiverOptions
	logger   *zap.Logger
	consumer consumer.Metrics
	runner   *interval.Runner
	rest     kubelet.RestClient
}

type receiverOptions struct {
	id                    config.ComponentID
	collectionInterval    time.Duration
	extraMetadataLabels   []kubelet.MetadataLabel
	metricGroupsToCollect map[kubelet.MetricGroup]bool
	k8sAPIClient          kubernetes.Interface
}

func newReceiver(rOptions *receiverOptions,
	logger *zap.Logger, rest kubelet.RestClient,
	next consumer.Metrics) *receiver {
	return &receiver{
		options:  rOptions,
		logger:   logger,
		consumer: next,
		rest:     rest,
	}
}

// Start the kubelet stats runnable.
func (r *receiver) Start(ctx context.Context, host component.Host) error {
	runnable := newRunnable(ctx, r.consumer, r.rest, r.logger, r.options)
	r.runner = interval.NewRunner(r.options.collectionInterval, runnable)

	go func() {
		if err := r.runner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

// Shutdown the kubelet stats runner.
func (r *receiver) Shutdown(ctx context.Context) error {
	r.runner.Stop()
	return nil
}
