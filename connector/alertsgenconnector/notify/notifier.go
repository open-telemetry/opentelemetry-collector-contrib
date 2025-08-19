package notify

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Notifier is responsible for sending alert notifications to external systems (e.g. Alertmanager).
type Notifier struct {
	logger *zap.Logger
	cfg    Config
}

// Config defines notification settings.
type Config struct {
	Endpoints   []string      `mapstructure:"endpoints"`
	Timeout     time.Duration `mapstructure:"timeout"`
	RetryPolicy string        `mapstructure:"retry_policy"`
}

// AlertEvent represents a single alert event ready for notification.
type AlertEvent struct {
	RuleID      string
	AlertName   string
	Severity    string
	Fingerprint string
	Labels      map[string]string
	Annotations map[string]string
	State       string // firing, resolved
	StartsAt    time.Time
	EndsAt      time.Time
	Generator   string
}

// New creates a new Notifier.
func New(set component.TelemetrySettings, cfg Config) *Notifier {
	return &Notifier{
		logger: set.Logger,
		cfg:    cfg,
	}
}

// Send sends a batch of alerts to configured endpoints.
func (n *Notifier) Send(ctx context.Context, alerts []AlertEvent) error {
	if len(alerts) == 0 {
		return nil
	}

	for _, ep := range n.cfg.Endpoints {
		n.logger.Info("Sending alerts", zap.String("endpoint", ep), zap.Int("count", len(alerts)))
		// TODO: Implement HTTP POST to Alertmanager or custom sink.
		// For now just simulate.
		for _, a := range alerts {
			n.logger.Debug("Alert", zap.String("rule_id", a.RuleID), zap.String("state", a.State))
		}
	}

	return nil
}

// SendError records and logs an error while notifying.
func (n *Notifier) SendError(alert AlertEvent, err error) {
	n.logger.Error("Failed to send alert",
		zap.String("rule_id", alert.RuleID),
		zap.String("alert", alert.AlertName),
		zap.Error(err),
	)
	fmt.Printf("Notifier error: %v\n", err)
}
