package alertsgenconnector

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const typeStr = "alertsgen"

func NewFactory() connector.Factory {
	return connector.NewFactory(
		typeStr,
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogs, component.StabilityLevelDevelopment),
		connector.WithTracesToMetrics(createTracesToMetrics, component.StabilityLevelDevelopment),
		connector.WithLogsToLogs(createLogsToLogs, component.StabilityLevelDevelopment),
		connector.WithLogsToMetrics(createLogsToMetrics, component.StabilityLevelDevelopment),
		connector.WithMetricsToMetrics(createMetricsToMetrics, component.StabilityLevelDevelopment),
		connector.WithMetricsToLogs(createMetricsToLogs, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ConnectorSettings: config.NewConnectorSettings(component.NewID(typeStr)),
		WindowSize:        5 * time.Second, // recommended default
		Storm: StormConfig{
			MaxAlertsPerMinute:      100,
			CircuitBreakerThreshold: 0.5,
		},
		Cardinality: CardinalityConfig{
			MaxLabelsPerAlert: 20,
			MaxSeriesPerRule:  1000,
			HashIfExceeds:     128,
		},
		Notify: NotifyConfig{
			Timeout: 5 * time.Second,
		},
	}
}

// Shared engine per ID
type engineHandle struct {
	ref int
	eng *Engine
}

var engines = map[component.ID]*engineHandle{}

func getOrCreateEngine(cfg *Config, set connector.CreateSettings) (*Engine, func(), error) {
	if h, ok := engines[set.ID]; ok {
		h.ref++
		return h.eng, func() { releaseEngine(set.ID) }, nil
	}
	eng, err := NewEngine(cfg, set)
	if err != nil {
		return nil, nil, err
	}
	engines[set.ID] = &engineHandle{ref: 1, eng: eng}
	return eng, func() { releaseEngine(set.ID) }, nil
}
func releaseEngine(id component.ID) {
	if h, ok := engines[id]; ok {
		h.ref--
		if h.ref == 0 {
			h.eng.Close()
			delete(engines, id)
		}
	}
}

// Traces->Logs
type t2l struct{ eng *Engine }

func createTracesToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Logs) (connector.TracesToLogs, error) {
	e, cleanup, err := getOrCreateEngine(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	e.SetLogsConsumer(next)
	return &t2l{eng: e}, nil
}
func (c *t2l) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *t2l) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *t2l) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.eng.ConsumeTraces(ctx, td)
}

// Traces->Metrics
type t2m struct{ eng *Engine }

func createTracesToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Metrics) (connector.TracesToMetrics, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	e.SetMetricsConsumer(next)
	return &t2m{eng: e}, nil
}
func (c *t2m) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *t2m) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *t2m) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.eng.ConsumeTraces(ctx, td)
}

// Logs->Logs
type l2l struct{ eng *Engine }

func createLogsToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Logs) (connector.LogsToLogs, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	e.SetLogsConsumer(next)
	return &l2l{eng: e}, nil
}
func (c *l2l) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *l2l) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *l2l) ConsumeLogs(ctx context.Context, ld plog.Logs) error  { return c.eng.ConsumeLogs(ctx, ld) }

// Logs->Metrics
type l2m struct{ eng *Engine }

func createLogsToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Metrics) (connector.LogsToMetrics, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	e.SetMetricsConsumer(next)
	return &l2m{eng: e}, nil
}
func (c *l2m) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *l2m) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *l2m) ConsumeLogs(ctx context.Context, ld plog.Logs) error  { return c.eng.ConsumeLogs(ctx, ld) }

// Metrics->Metrics
type m2m struct{ eng *Engine }

func createMetricsToMetrics(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Metrics) (connector.MetricsToMetrics, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	e.SetMetricsConsumer(next)
	return &m2m{eng: e}, nil
}
func (c *m2m) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *m2m) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *m2m) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return c.eng.ConsumeMetrics(ctx, md)
}

// Metrics->Logs
type m2l struct{ eng *Engine }

func createMetricsToLogs(_ context.Context, set connector.CreateSettings, cfg component.Config, next consumer.Logs) (connector.MetricsToLogs, error) {
	e, _, err := getOrCreateEngine(cfg.(*Config), set)
	if err != nil {
		return nil, err
	}
	e.SetLogsConsumer(next)
	return &m2l{eng: e}, nil
}
func (c *m2l) Start(ctx context.Context, host component.Host) error { return c.eng.Start(ctx, host) }
func (c *m2l) Shutdown(ctx context.Context) error                   { return c.eng.Shutdown(ctx) }
func (c *m2l) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return c.eng.ConsumeMetrics(ctx, md)
}
