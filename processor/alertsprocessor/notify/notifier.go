package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"

	"github.com/platformbuilds/alertsprocessor/processor/alertsprocessor"
	"github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/statestore"
)

type Notifier struct {
	cfg alertsprocessor.NotifierConfig
	log *zap.Logger
	cli *http.Client
	endpointHost string
}

func New(cfg alertsprocessor.NotifierConfig, log *zap.Logger) *Notifier {
	var host string
	if u, err := url.Parse(cfg.URL); err == nil { host = u.Host }
	return &Notifier{
		cfg: cfg,
		log: log,
		cli: &http.Client{Timeout: cfg.Timeout},
		endpointHost: host,
	}
}

func (n *Notifier) Notify(ctx context.Context, trans []statestore.Transition) {
	if n.cfg.DisableSending || n.cfg.URL == "" || len(trans) == 0 { return }
	now := time.Now().UTC()
	alerts := make([]AMAlert, 0, len(trans))
	for _, t := range trans {
		status := "firing"
		endsAt := ""
		if t.To == "resolved" { status = "resolved"; endsAt = now.Format(time.RFC3339Nano) }
		alerts = append(alerts, AMAlert{
			Status: status,
			Labels: t.Labels,
			StartsAt: t.At.UTC().Format(time.RFC3339Nano),
			EndsAt: endsAt,
		})
	}
	b, _ := json.Marshal(alerts)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.URL, bytes.NewReader(b))
	if err != nil { n.log.Warn("notifier: build request failed", zap.Error(err)); return }
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.cli.Do(req)
	if err != nil { n.log.Warn("notifier: send failed", zap.Error(err)); return }
	_ = resp.Body.Close()
}
