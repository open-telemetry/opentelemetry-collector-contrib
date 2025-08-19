package alertsgenconnector

import (
    "fmt"
    "time"

    "go.opentelemetry.io/collector/config"
)

type TSDBConfig struct {
    QueryURL string `mapstructure:"query_url"` // Prometheus-compatible /api/v1/query endpoint
    QueryInterval time.Duration `mapstructure:"query_interval"`
    TakeoverTimeout time.Duration `mapstructure:"takeover_timeout"`
}

type DedupConfig struct {
    FingerprintLabels []string `mapstructure:"fingerprint_labels"`
    ExcludeLabels []string `mapstructure:"exclude_labels"`
    Window time.Duration `mapstructure:"window"`
}

type StormConfig struct {
    MaxAlertsPerMinute int     `mapstructure:"max_alerts_per_minute"`
    CircuitBreakerThreshold float64 `mapstructure:"circuit_breaker_threshold"`
}

type CardinalityConfig struct {
    MaxLabelsPerAlert int `mapstructure:"max_labels_per_alert"`
    MaxSeriesPerRule  int `mapstructure:"max_series_per_rule"`
    HashIfExceeds     int `mapstructure:"hash_if_exceeds"`
}

type NotifyConfig struct {
    AlertmanagerURLs []string `mapstructure:"alertmanager_urls"`
    Timeout          time.Duration `mapstructure:"timeout"`
}

type Config struct {
    config.ConnectorSettings `mapstructure:",squash"`

    // Core
    WindowSize time.Duration `mapstructure:"window_size"` // recommended 5s
    InstanceID string        `mapstructure:"instance_id"`

    // Modules
    TSDB        TSDBConfig        `mapstructure:"tsdb"`
    Dedup       DedupConfig       `mapstructure:"dedup"`
    Storm       StormConfig       `mapstructure:"storm"`
    Cardinality CardinalityConfig `mapstructure:"cardinality"`
    Notify      NotifyConfig      `mapstructure:"notify"`

    // Rules
    Rules []RuleCfg `mapstructure:"rules"`
}

func (c *Config) Validate() error {
    if c.WindowSize <= 0 {
        return fmt.Errorf("window_size must be >0 (recommended 5s)")
    }
    if c.InstanceID == "" {
        return fmt.Errorf("instance_id must be set")
    }
    if c.TSDB.QueryURL == "" {
        // It's optional but recommended for HA sync; we won't fail hard.
    }
    return nil
}

// Rules & expressions
type RuleCfg struct {
    Name     string            `mapstructure:"name"`
    Signal   string            `mapstructure:"signal"`
    Select   map[string]string `mapstructure:"select"`
    GroupBy  []string          `mapstructure:"group_by"`
    Window   time.Duration     `mapstructure:"window"`
    Step     time.Duration     `mapstructure:"step"`
    For      time.Duration     `mapstructure:"for"`
    Severity string            `mapstructure:"severity"`
    Expr     ExprCfg           `mapstructure:"expr"`
    Outputs  OutputsCfg        `mapstructure:"outputs"`
    MaxGroups int              `mapstructure:"max_groups"`
}

type ExprCfg struct {
    Type     string  `mapstructure:"type"`
    Field    string  `mapstructure:"field"`
    Quantile float64 `mapstructure:"quantile"`
    Op       string  `mapstructure:"op"`
    Value    float64 `mapstructure:"value"`
}

type OutputsCfg struct {
    Log    *LogOutCfg    `mapstructure:"log"`
    Metric *MetricOutCfg `mapstructure:"metric"`
}

type LogOutCfg struct {
    BodyTemplate string            `mapstructure:"body_template"`
    Severity     string            `mapstructure:"severity"`
    Labels       map[string]string `mapstructure:"labels"`
}

type MetricOutCfg struct {
    Name   string            `mapstructure:"name"`
    Labels map[string]string `mapstructure:"labels"`
}
