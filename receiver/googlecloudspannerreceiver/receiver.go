// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudspannerreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

var _ component.MetricsReceiver = (*googleCloudSpannerReceiver)(nil)

type googleCloudSpannerReceiver struct {
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	config       *Config
	cancel       context.CancelFunc
}

func newGoogleCloudSpannerReceiver(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Metrics) (component.MetricsReceiver, error) {

	r := &googleCloudSpannerReceiver{
		logger:       logger,
		nextConsumer: nextConsumer,
		config:       config,
	}
	return r, nil
}

func (gcsReceiver *googleCloudSpannerReceiver) Start(ctx context.Context, host component.Host) error {
	ctx, gcsReceiver.cancel = context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(gcsReceiver.config.CollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// TODO Collect data here
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (gcsReceiver *googleCloudSpannerReceiver) Shutdown(context.Context) error {
	gcsReceiver.cancel()

	return nil
}
