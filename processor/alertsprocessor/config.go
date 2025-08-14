package alertsprocessor

import (
	"errors"
	"fmt"
	"time"

	evaluation "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/evaluation"
	"github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/stormcontrol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Sliding window over recent telemetry before evaluation.
type slidingWindowConfig struct {
	Duration         time.Duration `mapstructure:"duration"`          // e.g., "5s"
	MaxSamples       int           `mapstructure:"max_samples"`       // hard cap on samples
	OverflowBehavior string        `mapstructure:"overflow_behavior"` // ring_buffer | drop_new
}

// Periodic evaluation settings.
type evaluationConfig struct {
	Interval      time.Duration `mapstructure:"interval"`       // e.g., "15s"
	Timeout       time.Duration `mapstructure:"timeout"`        // per-eval timeout
	MaxConcurrent int           `mapstructure:"max_concurrent"` // reserved for future use
}

// Optional remote state sync; in-memory by default.
type statestoreConfig struct {
	RemoteRead struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"remote_read"`
	RemoteWrite struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"remote_write"`
	SyncInterval   time.Duration     `mapstructure:"sync_interval"` // e.g., "30s"
	InstanceID     string            `mapstructure:"instance_id"`
	ExternalLabels map[string]string `mapstructure:"external_labels"`
	ExternalURL    string            `mapstructure:"external_url"`
}

// Optional dedup/coalescing configuration (reserved for future use).
type dedupConfig struct {
	FingerprintAlgorithm string   `mapstructure:"fingerprint_algorithm"` // sha256, etc.
	FingerprintLabels    []string `mapstructure:"fingerprint_labels"`
	ExcludeLabels        []string `mapstructure:"exclude_labels"`
}

// Reuse the stormcontrol package's validated configuration directly.
// This avoids drift and type-mismatch at construction sites.
type stormControlConfig = stormcontrol.Config

// Cardinality controls.
type cardinalityLabels struct {
	MaxLabelsPerAlert   int `mapstructure:"max_labels_per_alert"`
	MaxLabelValueLength int `mapstructure:"max_label_value_length"`
	MaxTotalLabelSize   int `mapstructure:"max_total_label_size"`
}

type cardinalitySeries struct {
	MaxActiveSeries  int `mapstructure:"max_active_series"`
	MaxSeriesPerRule int `mapstructure:"max_series_per_rule"`
}

type cardinalityConfig struct {
	Labels        cardinalityLabels `mapstructure:"labels"`
	Allowlist     []string          `mapstructure:"allowlist"`
	Blocklist     []string          `mapstructure:"blocklist"`
	HashIfExceeds int               `mapstructure:"hash_if_exceeds"`
	HashAlgorithm string            `mapstructure:"hash_algorithm"`
	Series        cardinalitySeries `mapstructure:"series"`
	Enforcement   struct {
		Mode           string `mapstructure:"mode"`
		OverflowAction string `mapstructure:"overflow_action"`
	} `mapstructure:"enforcement"`
}

// Webhook notifier to emit Alertmanager-like payloads.
type notifierConfig struct {
	URL             string                  `mapstructure:"url"`
	HTTPClient      confighttp.ClientConfig `mapstructure:",squash"`
	Timeout         time.Duration           `mapstructure:"timeout"`          // HTTP request timeout
	InitialInterval time.Duration           `mapstructure:"initial_interval"` // retry/backoff start
	MaxInterval     time.Duration           `mapstructure:"max_interval"`     // retry/backoff cap
	MaxBatchSize    int                     `mapstructure:"max_batch_size"`
	DisableSending  bool                    `mapstructure:"disable_sending"`
}

type ruleFiles struct {
	Include []string `mapstructure:"include"`
}

type Config struct {
	// NOTE: do not embed config.ProcessorSettings in modern Collector.
	SlidingWindow slidingWindowConfig `mapstructure:"sliding_window"`
	Evaluation    evaluationConfig    `mapstructure:"evaluation"`
	Statestore    statestoreConfig    `mapstructure:"statestore"`
	Dedup         dedupConfig         `mapstructure:"deduplication"`
	StormControl  stormControlConfig  `mapstructure:"stormcontrol"`
	Cardinality   cardinalityConfig   `mapstructure:"cardinality"`
	Notifier      notifierConfig      `mapstructure:"notifier"`

	RuleFiles ruleFiles         `mapstructure:"rule_files"`
	Rules     []evaluation.Rule `mapstructure:"rules"`
}

func createDefaultConfig() component.Config {
	return &Config{
		SlidingWindow: slidingWindowConfig{
			Duration:         5 * time.Second,
			MaxSamples:       100_000,
			OverflowBehavior: "ring_buffer",
		},
		Evaluation: evaluationConfig{
			Interval:      15 * time.Second,
			Timeout:       10 * time.Second,
			MaxConcurrent: 0,
		},
		Statestore: statestoreConfig{
			SyncInterval: 30 * time.Second,
		},
		// Sensible storm governor defaults (EMA/backoff between 1s..30s).
		StormControl: stormcontrol.Config{
			MinInterval:             stormcontrol.Duration(1 * time.Second),
			MaxInterval:             stormcontrol.Duration(30 * time.Second),
			BackoffFactor:           2.0,
			RecoverFactor:           0.5,
			MaxActiveAlerts:         1000,
			MaxAlertsPerMinute:      2000,
			CircuitBreakerThreshold: 0,
		},
		Notifier: notifierConfig{
			Timeout:         5 * time.Second,
			InitialInterval: 500 * time.Millisecond,
			MaxInterval:     30 * time.Second,
			MaxBatchSize:    64,
		},
		RuleFiles: ruleFiles{},
		Rules:     nil,
	}
}

func (c *Config) Validate() error {
	// Sliding window.
	if c.SlidingWindow.Duration <= 0 {
		return errors.New("sliding_window.duration must be > 0")
	}
	if c.SlidingWindow.OverflowBehavior != "ring_buffer" && c.SlidingWindow.OverflowBehavior != "drop_new" {
		return fmt.Errorf("sliding_window.overflow_behavior must be ring_buffer or drop_new")
	}

	// Evaluation cadence.
	if c.Evaluation.Interval <= 0 {
		return errors.New("evaluation.interval must be > 0")
	}
	if c.Evaluation.Timeout <= 0 {
		return errors.New("evaluation.timeout must be > 0")
	}
	if c.Evaluation.Timeout > c.Evaluation.Interval {
		return fmt.Errorf("evaluation.timeout (%v) must be <= evaluation.interval (%v)", c.Evaluation.Timeout, c.Evaluation.Interval)
	}

	// Notifier (when enabled).
	if c.Notifier.MaxBatchSize < 0 {
		return errors.New("notifier.max_batch_size must be >= 0")
	}
	if c.Notifier.InitialInterval < 0 || c.Notifier.MaxInterval < 0 || c.Notifier.Timeout < 0 {
		return errors.New("notifier intervals/timeouts must be >= 0")
	}
	if c.Notifier.MaxInterval > 0 && c.Notifier.InitialInterval > c.Notifier.MaxInterval {
		return fmt.Errorf("notifier.initial_interval (%v) must be <= notifier.max_interval (%v)", c.Notifier.InitialInterval, c.Notifier.MaxInterval)
	}

	// Storm-control config (full semantic validation).
	if _, err := c.StormControl.Validate(); err != nil {
		return fmt.Errorf("stormcontrol: %w", err)
	}

	return nil
}
