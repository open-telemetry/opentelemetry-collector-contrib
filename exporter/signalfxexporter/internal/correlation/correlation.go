// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/tracetracker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// Tracker correlation
type Tracker struct {
	once         sync.Once
	log          *zap.Logger
	cfg          *Config
	params       exporter.Settings
	traceTracker *tracetracker.ActiveServiceTracker
	pTicker      timeutils.TTicker
	correlation  *correlationContext
	accessToken  configopaque.String
}

type correlationContext struct {
	correlations.CorrelationClient
	cancel context.CancelFunc
}

// NewTracker creates a new tracker instance for correlation.
func NewTracker(cfg *Config, accessToken configopaque.String, params exporter.Settings) *Tracker {
	return &Tracker{
		log:         params.Logger,
		cfg:         cfg,
		params:      params,
		accessToken: accessToken,
	}
}

func newCorrelationClient(ctx context.Context, cfg *Config, accessToken configopaque.String, params exporter.Settings, host component.Host) (
	*correlationContext, error,
) {
	corrURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse correlation endpoint URL %q: %w", cfg.Endpoint, err)
	}

	httpClient, err := cfg.ToClient(ctx, host, params.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create correlation API client: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	client, err := correlations.NewCorrelationClient(ctx, newZapShim(params.Logger), httpClient, correlations.ClientConfig{
		Config:      cfg.Config,
		AccessToken: string(accessToken),
		URL:         corrURL,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create correlation client: %w", err)
	}

	return &correlationContext{
		CorrelationClient: client,
		cancel:            cancel,
	}, nil
}

// ProcessTraces processes the provided spans to correlate the services and environment observed
// to the resources (host, pods, etc.) emitting the spans.
func (cor *Tracker) ProcessTraces(ctx context.Context, traces ptrace.Traces) error {
	if cor == nil || traces.ResourceSpans().Len() == 0 {
		return nil
	}

	cor.once.Do(func() {
		res := traces.ResourceSpans().At(0).Resource()
		hostID, ok := splunk.ResourceToHostID(res)

		if !ok {
			cor.log.Warn("Unable to determine host resource ID for correlation syncing")
			return
		}
		cor.log.Info("Detected host resource ID for correlation", zap.Any("hostID", hostID))

		hostDimension := string(hostID.Key)

		cor.traceTracker = tracetracker.New(
			newZapShim(cor.params.Logger),
			cor.cfg.StaleServiceTimeout,
			cor.correlation,
			map[string]string{
				hostDimension: hostID.ID,
			},
			cor.cfg.SyncAttributes)

		cor.pTicker = &timeutils.PolicyTicker{OnTickFunc: cor.traceTracker.Purge}
		cor.pTicker.Start(cor.cfg.StaleServiceTimeout)

		cor.correlation.Start()
	})

	if cor.traceTracker != nil {
		cor.traceTracker.ProcessTraces(ctx, traces)
	}

	return nil
}

// Start correlation tracking.
func (cor *Tracker) Start(ctx context.Context, host component.Host) (err error) {
	cor.correlation, err = newCorrelationClient(ctx, cor.cfg, cor.accessToken, cor.params, host)
	if err != nil {
		return err
	}

	return nil
}

// Shutdown correlation tracking.
func (cor *Tracker) Shutdown(_ context.Context) error {
	if cor != nil {
		if cor.correlation != nil {
			cor.correlation.cancel()
			cor.correlation.Shutdown()
		}

		if cor.pTicker != nil {
			cor.pTicker.Stop()
		}
	}
	return nil
}
