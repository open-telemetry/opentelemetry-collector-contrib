// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/notify"

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/statestore"
)

// Config is a local, minimal notifier configuration to avoid importing the parent package.
type Config struct {
	URL             string
	Timeout         time.Duration
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxBatchSize    int
	DisableSending  bool
}

type Notifier struct {
	cfg          Config
	log          *zap.Logger
	cli          *http.Client
	endpointHost string
}

func New(cfg Config, log *zap.Logger) *Notifier {
	var host string
	if u, err := url.Parse(cfg.URL); err == nil {
		host = u.Host
	}
	return &Notifier{
		cfg:          cfg,
		log:          log,
		cli:          &http.Client{Timeout: cfg.Timeout},
		endpointHost: host,
	}
}

// Notify sends a batch of state transitions to the configured endpoint.
// No retries/backoff here yet—kept intentionally simple for first cut.
func (n *Notifier) Notify(ctx context.Context, trans []statestore.Transition) {
	if n.cfg.DisableSending || n.cfg.URL == "" || len(trans) == 0 {
		return
	}
	now := time.Now().UTC()

	payload := make([]AMAlert, 0, len(trans))
	for _, t := range trans {
		status := "firing"
		endsAt := ""
		if t.To == "resolved" {
			status = "resolved"
			endsAt = now.Format(time.RFC3339Nano)
		}
		payload = append(payload, AMAlert{
			Status:   status,
			Labels:   t.Labels,
			StartsAt: t.At.UTC().Format(time.RFC3339Nano),
			EndsAt:   endsAt,
		})
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.URL, bytes.NewReader(body))
	if err != nil {
		if n.log != nil {
			n.log.Warn("notifier: building request failed", zap.Error(err))
		}
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.cli.Do(req)
	if err != nil {
		if n.log != nil {
			n.log.Warn("notifier: send failed", zap.Error(err))
		}
		return
	}
	_ = resp.Body.Close()
}
