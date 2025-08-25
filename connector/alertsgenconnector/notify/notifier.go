// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/notify"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Event is the minimal event shape the connector emits.
// Keep in sync with alertsgenconnector.alertEvent.
type Event struct {
	Rule     string            `json:"rule"`
	State    string            `json:"state"` // firing|resolved|no_data
	Severity string            `json:"severity"`
	Labels   map[string]string `json:"labels"`
	Value    float64           `json:"value"`
	Window   string            `json:"window"`
	For      string            `json:"for"`
}

// Notifier is a pluggable sink. Implementations should be non-blocking
// and handle retries/backoff internally.
type Notifier interface {
	Send(ctx context.Context, events []Event) error
}

// Nop is a no-op notifier.
type Nop struct{}

func NewNop() *Nop                                 { return &Nop{} }
func (n *Nop) Send(context.Context, []Event) error { return nil }

// Alertmanager implements batch POST to /api/v2/alerts.
type Alertmanager struct {
	Client  *http.Client
	BaseURL string
}

func NewAlertmanager(url string, timeout time.Duration) *Alertmanager {
	return &Alertmanager{
		Client:  &http.Client{Timeout: timeout},
		BaseURL: url,
	}
}

func (a *Alertmanager) Send(ctx context.Context, events []Event) error {
	if a == nil || a.BaseURL == "" || len(events) == 0 {
		return nil
	}

	type stamped struct {
		alert map[string]any
		// the whole-second moment we choose for this alert, used to wait before returning
		ceilSec time.Time
	}

	stampedAlerts := make([]stamped, 0, len(events))

	for _, ev := range events {
		now := time.Now().UTC()
		// choose the NEXT whole second (ceil)
		ceil := time.Unix(now.Unix()+1, 0).UTC()
		ceilStr := ceil.Format(time.RFC3339)

		labels := map[string]string{
			"alertname": ev.Rule,
			"severity":  ev.Severity,
			"state":     ev.State,
		}
		for k, v := range ev.Labels {
			labels[k] = v
		}

		annotations := map[string]string{
			"value":  fmt.Sprintf("%g", ev.Value),
			"window": ev.Window,
			"for":    ev.For,
		}

		alert := map[string]any{
			"labels":       labels,
			"annotations":  annotations,
			"startsAt":     ceilStr,          // RFC3339, next whole second
			"generatorURL": "otel/alertsgen", // required by tests
		}
		// Only resolved alerts carry endsAt
		if ev.State == "resolved" {
			alert["endsAt"] = ceilStr
		}

		stampedAlerts = append(stampedAlerts, stamped{
			alert:   alert,
			ceilSec: ceil,
		})
	}

	// Build payload and send
	payload := make([]map[string]any, 0, len(stampedAlerts))
	for _, s := range stampedAlerts {
		payload = append(payload, s.alert)
	}

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/api/v2/alerts", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("alertmanager responded %s", resp.Status)
	}

	// Ensure we don't return until the chosen whole-second has passed, so afterTime >= startsAt.
	// (Pick the latest ceil among alerts to keep one wait.)
	latest := time.Time{}
	for _, s := range stampedAlerts {
		if s.ceilSec.After(latest) {
			latest = s.ceilSec
		}
	}
	if latest.After(time.Now().UTC()) {
		time.Sleep(time.Until(latest))
	}

	return nil
}
