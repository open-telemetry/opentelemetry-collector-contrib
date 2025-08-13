package alertsprocessor

import (
	"errors"
	"fmt"
	"time"

	evaluation "github.com/platformbuilds/alertsprocessor/processor/alertsprocessor/evaluation"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
)

type SlidingWindowConfig struct {
	Duration         time.Duration `mapstructure:"duration"`
	MaxSamples       int           `mapstructure:"max_samples"`
	OverflowBehavior string        `mapstructure:"overflow_behavior"` // ring_buffer | drop_new
}

type EvaluationConfig struct {
	Interval      time.Duration `mapstructure:"interval"`
	Timeout       time.Duration `mapstructure:"timeout"`
	MaxConcurrent int           `mapstructure:"max_concurrent"`
}

type StatestoreConfig struct {
	RemoteRead struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"remote_read"`
	RemoteWrite struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"remote_write"`
	SyncInterval   time.Duration     `mapstructure:"sync_interval"`
	InstanceID     string            `mapstructure:"instance_id"`
	ExternalLabels map[string]string `mapstructure:"external_labels"`
	ExternalURL    string            `mapstructure:"external_url"`
}

type DedupConfig struct {
	FingerprintAlgorithm string   `mapstructure:"fingerprint_algorithm"` // sha256
	FingerprintLabels    []string `mapstructure:"fingerprint_labels"`
	ExcludeLabels        []string `mapstructure:"exclude_labels"`
}

type StormControlConfig struct {
	Global struct {
		MaxActiveAlerts         int     `mapstructure:"max_active_alerts"`
		MaxAlertsPerMinute      int     `mapstructure:"max_alerts_per_minute"`
		CircuitBreakerThreshold float64 `mapstructure:"circuit_breaker_threshold"`
	} `mapstructure:"global"`
}

type CardinalityLabels struct {
	MaxLabelsPerAlert   int `mapstructure:"max_labels_per_alert"`
	MaxLabelValueLength int `mapstructure:"max_label_value_length"`
	MaxTotalLabelSize   int `mapstructure:"max_total_label_size"`
}

type CardinalitySeries struct {
	MaxActiveSeries  int `mapstructure:"max_active_series"`
	MaxSeriesPerRule int `mapstructure:"max_series_per_rule"`
}

type CardinalityConfig struct {
	Labels        CardinalityLabels `mapstructure:"labels"`
	Allowlist     []string          `mapstructure:"allowlist"`
	Blocklist     []string          `mapstructure:"blocklist"`
	HashIfExceeds int               `mapstructure:"hash_if_exceeds"`
	HashAlgorithm string            `mapstructure:"hash_algorithm"`
	Series        CardinalitySeries `mapstructure:"series"`
	Enforcement   struct {
		Mode           string `mapstructure:"mode"`
		OverflowAction string `mapstructure:"overflow_action"`
	} `mapstructure:"enforcement"`
}

type NotifierConfig struct {
	URL             string                  `mapstructure:"url"`
	HTTPClient      confighttp.ClientConfig `mapstructure:",squash"`
	Timeout         time.Duration           `mapstructure:"timeout"`
	InitialInterval time.Duration           `mapstructure:"initial_interval"`
	MaxInterval     time.Duration           `mapstructure:"max_interval"`
	MaxBatchSize    int                     `mapstructure:"max_batch_size"`
	DisableSending  bool                    `mapstructure:"disable_sending"`
}

type RuleFiles struct {
	Include []string `mapstructure:"include"`
}

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	SlidingWindow SlidingWindowConfig `mapstructure:"sliding_window"`
	Evaluation    EvaluationConfig    `mapstructure:"evaluation"`
	Statestore    StatestoreConfig    `mapstructure:"statestore"`
	Dedup         DedupConfig         `mapstructure:"deduplication"`
	StormControl  StormControlConfig  `mapstructure:"stormcontrol"`
	Cardinality   CardinalityConfig   `mapstructure:"cardinality"`
	Notifier      NotifierConfig      `mapstructure:"notifier"`

	RuleFiles RuleFiles         `mapstructure:"rule_files"`
	Rules     []evaluation.Rule `mapstructure:"rules"` // inline rules
}

func createDefaultConfig() component.Config {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(component.MustNewID(typeStr)),
		SlidingWindow:     SlidingWindowConfig{Duration: 5 * time.Second, MaxSamples: 100_000, OverflowBehavior: "ring_buffer"},
		Evaluation:        EvaluationConfig{Interval: 15 * time.Second, Timeout: 10 * time.Second, MaxConcurrent: 0},
		Statestore:        StatestoreConfig{SyncInterval: 30 * time.Second},
		Notifier:          NotifierConfig{Timeout: 5 * time.Second, InitialInterval: 500 * time.Millisecond, MaxInterval: 30 * time.Second, MaxBatchSize: 64},
		RuleFiles:         RuleFiles{Include: nil},
		Rules:             nil,
	}
}

func (c *Config) Validate() error {
	if c.SlidingWindow.Duration <= 0 {
		return errors.New("sliding_window.duration must be > 0")
	}
	if c.Evaluation.Interval <= 0 {
		return errors.New("evaluation.interval must be > 0")
	}
	if c.SlidingWindow.OverflowBehavior != "ring_buffer" && c.SlidingWindow.OverflowBehavior != "drop_new" {
		return fmt.Errorf("sliding_window.overflow_behavior must be ring_buffer or drop_new")
	}
	return nil
}
