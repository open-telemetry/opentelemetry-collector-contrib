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
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ component.MetricsReceiver = (*receiver)(nil)

type receiver struct {
	options  *receiverOptions
	logger   *zap.Logger
	consumer consumer.MetricsConsumer
	runner   *interval.Runner
	rest     kubelet.RestClient
}

type receiverOptions struct {
	name                  string
	collectionInterval    time.Duration
	extraMetadataLabels   []kubelet.MetadataLabel
	metricGroupsToCollect map[kubelet.MetricGroup]bool
	k8sAPIClient          kubernetes.Interface
}

func newReceiver(rOptions *receiverOptions,
	logger *zap.Logger, rest kubelet.RestClient,
	next consumer.MetricsConsumer) (*receiver, error) {
	return &receiver{
		options:  rOptions,
		logger:   logger,
		consumer: next,
		rest:     rest,
	}, nil
}

// Creates and starts the kubelet stats runnable.
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

// Stops the kubelet stats runner.
func (r *receiver) Shutdown(ctx context.Context) error {
	r.runner.Stop()
	return nil
}
