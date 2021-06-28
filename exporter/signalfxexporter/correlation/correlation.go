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

package correlation

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"github.com/signalfx/signalfx-agent/pkg/apm/tracetracker"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// Tracker correlation
type Tracker struct {
	once         sync.Once
	log          *zap.Logger
	cfg          *Config
	params       component.ExporterCreateSettings
	traceTracker *tracetracker.ActiveServiceTracker
	correlation  *correlationContext
	accessToken  string
}

type correlationContext struct {
	correlations.CorrelationClient
	cancel context.CancelFunc
}

// NewTracker creates a new tracker instance for correlation.
func NewTracker(cfg *Config, accessToken string, params component.ExporterCreateSettings) *Tracker {
	return &Tracker{
		log:         params.Logger,
		cfg:         cfg,
		params:      params,
		accessToken: accessToken,
	}
}

func newCorrelationClient(cfg *Config, accessToken string, params component.ExporterCreateSettings, host component.Host) (
	*correlationContext, error,
) {
	corrURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse correlation endpoint URL %q: %v", cfg.Endpoint, err)
	}

	httpClient, err := cfg.ToClient(host.GetExtensions())
	if err != nil {
		return nil, fmt.Errorf("failed to create correlation API client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client, err := correlations.NewCorrelationClient(newZapShim(params.Logger), ctx, httpClient, correlations.ClientConfig{
		Config:      cfg.Config,
		AccessToken: accessToken,
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
func (cor *Tracker) AddSpans(ctx context.Context, traces pdata.Traces) error {
	if cor == nil || traces.ResourceSpans().Len() == 0 {
		return nil
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

		hostDimension := string(hostID.Key)

		cor.traceTracker = tracetracker.New(
			newZapShim(cor.params.Logger),
			cor.cfg.StaleServiceTimeout,
			cor.correlation,
			map[string]string{
				hostDimension: hostID.ID,
			},
			false,
			nil,
			cor.cfg.SyncAttributes)

		cor.correlation.Start()
	})

	if cor.traceTracker != nil {
		cor.traceTracker.AddSpansGeneric(ctx, spanListWrap{traces.ResourceSpans()})
	}

	return nil
}

// Start correlation tracking.
func (cor *Tracker) Start(_ context.Context, host component.Host) (err error) {
	cor.correlation, err = newCorrelationClient(cor.cfg, cor.accessToken, cor.params, host)
	if err != nil {
		return err
	}

	return nil
}

// Shutdown correlation tracking.
func (cor *Tracker) Shutdown(_ context.Context) error {
	if cor != nil && cor.correlation != nil {
		cor.correlation.cancel()
	}
	return nil
}
