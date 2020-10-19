// Copyright The OpenTelemetry Authors
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

package sapmexporter

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"github.com/signalfx/signalfx-agent/pkg/apm/tracetracker"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

// Tracker correlation
type Tracker struct {
	once         sync.Once
	log          *zap.Logger
	cfg          *Config
	params       component.ExporterCreateParams
	traceTracker *tracetracker.ActiveServiceTracker
	correlation  *correlationContext
}

type correlationContext struct {
	correlations.CorrelationClient
	cancel context.CancelFunc
}

// NewTracker creates a new tracker instance for correlation.
func NewTracker(cfg *Config, params component.ExporterCreateParams) *Tracker {
	return &Tracker{
		log:    params.Logger,
		cfg:    cfg,
		params: params,
	}
}

func newCorrelationClient(cfg *Config, params component.ExporterCreateParams) (
	*correlationContext, error,
) {
	corrURL, err := url.Parse(cfg.Correlation.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse correlation endpoint URL %q: %v", cfg.Correlation.Endpoint, err)
	}

	httpClient, err := cfg.Correlation.ToClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create correlation API client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client, err := correlations.NewCorrelationClient(newZapShim(params.Logger), ctx, httpClient, correlations.ClientConfig{
		Config:      cfg.Correlation.Config,
		AccessToken: cfg.AccessToken,
		URL:         corrURL,
	})

	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create correlation client: %v", err)
	}

	return &correlationContext{
		CorrelationClient: client,
		cancel:            cancel,
	}, nil
}

// AddSpans processes the provided spans to correlate the services and environment observed
// to the resources (host, pods, etc.) emitting the spans.
func (cor *Tracker) AddSpans(ctx context.Context, traces pdata.Traces) {
	if cor == nil || traces.ResourceSpans().Len() == 0 {
		return
	}

	cor.once.Do(func() {
		res := traces.ResourceSpans().At(0).Resource()
		hostID, ok := splunk.ResourceToHostID(res)

		if ok {
			cor.log.Info("Detected host resource ID for correlation", zap.Any("hostID", hostID))
		} else {
			cor.log.Warn("Unable to determine host resource ID for correlation syncing")
			return
		}

		var err error
		cor.correlation, err = newCorrelationClient(cor.cfg, cor.params)
		if err != nil {
			cor.log.Error("Failed to create correlation client", zap.Error(err))
			return
		}

		cor.traceTracker = tracetracker.New(
			newZapShim(cor.params.Logger),
			cor.cfg.Correlation.StaleServiceTimeout,
			cor.correlation,
			map[string]string{
				string(hostID.Key): hostID.ID,
			},
			false,
			nil,
			cor.cfg.Correlation.SyncAttributes)
		cor.Start()
	})

	if cor.traceTracker != nil {
		cor.traceTracker.AddSpansGeneric(ctx, spanListWrap{traces.ResourceSpans()})
	}
}

// Start correlation tracking.
func (cor *Tracker) Start() {
	if cor != nil && cor.correlation != nil {
		cor.correlation.Start()
	}
}

// Shutdown correlation tracking.
func (cor *Tracker) Shutdown() {
	if cor != nil && cor.correlation != nil {
		cor.correlation.cancel()
	}
}
