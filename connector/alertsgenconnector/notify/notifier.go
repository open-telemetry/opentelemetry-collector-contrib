
package notify

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Config for Notifier
type Config struct {
	Endpoints []string
	Timeout   time.Duration
}

// AlertEvent is a simplified notification payload.
type AlertEvent struct {
	Rule     string
	State    string
	Severity string
	Labels   map[string]string
	Value    float64
	Window   string
	For      string
}

// Notifier sends alerts to endpoints (Alertmanager-compatible hook to be implemented).
type Notifier struct {
	logger *zap.Logger
	cfg    Config
}

// New constructs a Notifier.
func New(set component.TelemetrySettings, cfg Config) *Notifier {
	return &Notifier{logger: set.Logger, cfg: cfg}
}

// Notify sends a batch of alert events (best-effort).
func (n *Notifier) Notify(alerts []AlertEvent) error {
	if len(alerts) == 0 {
		return nil
	}
	// For production, implement HTTP POST to n.cfg.Endpoints with n.cfg.Timeout.
	// Here we just log them for now.
	for _, a := range alerts {
		n.logger.Info("alert",
			zap.String("rule", a.Rule),
			zap.String("state", a.State),
			zap.String("severity", a.Severity),
		)
	}
	return nil
}

// SendError helper for logging errors.
func (n *Notifier) SendError(ctx context.Context, err error, endpoint string) {
	n.logger.Error("notify failed", zap.String("endpoint", endpoint), zap.Error(err))
}
