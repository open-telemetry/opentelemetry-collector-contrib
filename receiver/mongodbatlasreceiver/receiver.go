// Copyright  OpenTelemetry Authors
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

package mongodbatlasreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

type receiver struct {
	ctx        context.Context
	log        *zap.Logger
	cfg        *Config
	metricSink consumer.Metrics
	client     *internal.MongoDBAtlasClient
	lastRun    *time.Time
	runFreq    time.Duration
	runner     *interval.Runner
}

func newMongoDBAtlasReceiver(
	ctx context.Context,
	log *zap.Logger,
	cfg *Config,
	sink consumer.Metrics,
) (*receiver, error) {
	client, err := internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, log)
	if err != nil {
		return nil, err
	}
	return &receiver{ctx, log, cfg, sink, client, nil, time.Minute, nil}, nil
}

func (s *receiver) Setup() error {
	return nil
}

func (s *receiver) Start(ctx context.Context, host component.Host) error {
	s.runner = interval.NewRunner(s.runFreq, s)
	go func() {
		if err := s.runner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (s *receiver) Run() error {
	var start time.Time
	if s.lastRun != nil {
		start = *s.lastRun
	} else {
		start = time.Now().Add(s.runFreq * -1)
	}
	now := time.Now()
	s.poll(start.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), s.cfg.Granularity)
	s.lastRun = &now
	return nil
}

func (s *receiver) Shutdown(ctx context.Context) error {
	if s.runner != nil {
		s.runner.Stop()
	}
	return nil
}

func (s *receiver) poll(start string, end string, resolution string) { //nolint
	ctx := s.ctx
	for _, org := range s.client.Organizations(ctx) {
		// Metrics collection starts here
		s.log.Debug("fetch resources for MongoDB Organization", zap.String("org", org.Name))
	}
}
