
package alertsgenconnector

import (
	"fmt"
	"time"
)

type TSDBConfig struct {
	QueryURL        string        `mapstructure:"query_url"`
	QueryInterval   time.Duration `mapstructure:"query_interval"`
	TakeoverTimeout time.Duration `mapstructure:"takeover_timeout"`
}

type DedupConfig struct {
	Window            time.Duration `mapstructure:"window"`
	FingerprintLabels []string      `mapstructure:"fingerprint_labels"`
	ExcludeLabels     []string      `mapstructure:"exclude_labels"`
}

type StormConfig struct {
	MaxAlertsPerMinute      int     `mapstructure:"max_alerts_per_minute"`
	CircuitBreakerThreshold float64 `mapstructure:"circuit_breaker_threshold"`
}

type CardinalityConfig struct {
	MaxLabelsPerAlert int `mapstructure:"max_labels_per_alert"`
	MaxSeriesPerRule  int `mapstructure:"max_series_per_rule"`
	HashIfExceeds     int `mapstructure:"hash_if_exceeds"`
}

type NotifyConfig struct {
	AlertmanagerURLs []string      `mapstructure:"alertmanager_urls"`
	Timeout          time.Duration `mapstructure:"timeout"`
}

type RuleCfg struct {
	Name     string            `mapstructure:"name"`
	Signal   string            `mapstructure:"signal"` // traces|logs|metrics
	Select   map[string]string `mapstructure:"select"`
	Severity string            `mapstructure:"severity"`
	GroupBy  []string          `mapstructure:"group_by"`

	Window time.Duration `mapstructure:"window"`
	Step   time.Duration `mapstructure:"step"`
	For    time.Duration `mapstructure:"for"`

	Expr ExprCfg `mapstructure:"expr"`
}

type ExprCfg struct {
	Type     string  `mapstructure:"type"`
	Field    string  `mapstructure:"field"`
	Quantile float64 `mapstructure:"quantile"`
	Op       string  `mapstructure:"op"`
	Value    float64 `mapstructure:"value"`
}

type Config struct {
	// Core
	WindowSize time.Duration `mapstructure:"window_size"`
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
		return fmt.Errorf("window_size must be > 0")
	}
	if len(c.Rules) == 0 {
		return fmt.Errorf("at least one rule is required")
	}
	return nil
}
