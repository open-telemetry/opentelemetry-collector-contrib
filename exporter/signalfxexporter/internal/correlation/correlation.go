// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"github.com/signalfx/signalfx-agent/pkg/apm/tracetracker"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// Tracker correlation
type Tracker struct {
	once         sync.Once
	log          *zap.Logger
	cfg          *Config
	params       exporter.CreateSettings
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
func NewTracker(cfg *Config, accessToken configopaque.String, params exporter.CreateSettings) *Tracker {
	return &Tracker{
		log:         params.Logger,
		cfg:         cfg,
		params:      params,
		accessToken: accessToken,
	}
}

func newCorrelationClient(cfg *Config, accessToken configopaque.String, params exporter.CreateSettings, host component.Host) (
	*correlationContext, error,
) {
	corrURL, err := url.Parse(cfg.HTTPClientSettings.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse correlation endpoint URL %q: %w", cfg.HTTPClientSettings.Endpoint, err)
	}

	httpClient, err := cfg.ToClient(host, params.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create correlation API client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client, err := correlations.NewCorrelationClient(newZapShim(params.Logger), ctx, httpClient, correlations.ClientConfig{
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

// AddSpans processes the provided spans to correlate the services and environment observed
// to the resources (host, pods, etc.) emitting the spans.
func (cor *Tracker) AddSpans(ctx context.Context, traces ptrace.Traces) error {
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

		cor.pTicker = &timeutils.PolicyTicker{OnTickFunc: cor.traceTracker.Purge}
		cor.pTicker.Start(cor.cfg.StaleServiceTimeout)

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
	if cor != nil {
		if cor.correlation != nil {
			cor.correlation.cancel()
		}

		if cor.pTicker != nil {
			cor.pTicker.Stop()
		}
	}
	return nil
}
