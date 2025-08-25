// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config is the top-level connector configuration.
type Config struct {
	// Sliding evaluation window (default 5s).
	WindowSize time.Duration `mapstructure:"window"`

	// Step/interval to run evaluations (default = window).
	Step time.Duration `mapstructure:"step"`

	// Unique identifier for this collector instance (for HA coordination).
	InstanceID string `mapstructure:"instance_id"`

	// Rules to evaluate.
	Rules []RuleCfg `mapstructure:"rules"`

	// HA/TSDB integration for state restore and coordination.
	TSDB *TSDBConfig `mapstructure:"tsdb"`

	// De-duplication configuration.
	Dedup DedupConfig `mapstructure:"dedup"`

	// Limits across storm-control/cardinality.
	Limits LimitsConfig `mapstructure:"limits"`

	// Optional notifications behavior.
	Notify NotifyConfig `mapstructure:"notify"`

	// Memory management configuration.
	Memory MemoryConfig `mapstructure:"memory"`

	// NEW: also emit a metric per alert event produced (default true).
	EmitAlertMetrics bool `mapstructure:"emit_alert_metrics"`
}

// TSDBConfig configures interactions with a time series DB (Prometheus/VictoriaMetrics, etc.)
type TSDBConfig struct {
	// NEW: when false, TSDB is optional and not required by tests/local.
	Enabled bool `mapstructure:"enabled"`

	// Base query URL, e.g., http://prometheus:9090
	QueryURL string `mapstructure:"query_url"`

	// Optional remote write URL to push metrics.
	RemoteWriteURL string `mapstructure:"remote_write_url"`

	// Enable remote write.
	EnableRemoteWrite bool `mapstructure:"enable_remote_write"`

	// Request timeouts.
	QueryTimeout time.Duration `mapstructure:"query_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`

	// De-duplication window for HA replicas.
	DedupWindow time.Duration `mapstructure:"dedup_window"`

	// Remote write batching.
	RemoteWriteBatchSize     int           `mapstructure:"remote_write_batch_size"`
	RemoteWriteFlushInterval time.Duration `mapstructure:"remote_write_flush_interval"`
}

// DedupConfig configures label-based fingerprinting and de-dup behavior.
type DedupConfig struct {
	FingerprintLabels []string      `mapstructure:"fingerprint_labels"`
	ExcludeLabels     []string      `mapstructure:"exclude_labels"`
	Window            time.Duration `mapstructure:"window"`
	EnableTSDBDedup   bool          `mapstructure:"enable_tsdb_dedup"`
}

// LimitsConfig aggregates storm-control and cardinality limits.
type LimitsConfig struct {
	Storm       StormCfg       `mapstructure:"storm"`
	Cardinality CardinalityCfg `mapstructure:"cardinality"`
}

// StormCfg controls alert storms, state machine flapping, and circuit breaking.
type StormCfg struct {
	MaxTransitionsPerMinute    int           `mapstructure:"max_transitions_per_minute"`
	MaxEventsPerInterval       int           `mapstructure:"max_events_per_interval"`
	Interval                   time.Duration `mapstructure:"interval"`
	CircuitBreakerThreshold    float64       `mapstructure:"circuit_breaker_threshold"`
	CircuitBreakerRecoveryTime time.Duration `mapstructure:"circuit_breaker_recovery_time"`
}

// CardinalityCfg constrains label explosion and memory footprint.
type CardinalityCfg struct {
	MaxLabels            int            `mapstructure:"max_labels"`
	MaxValLen            int            `mapstructure:"max_label_value_length"`
	MaxTotalSize         int            `mapstructure:"max_total_label_size"`
	HashIfExceeds        int            `mapstructure:"hash_if_exceeds"`
	Allow                []string       `mapstructure:"allowlist"`
	Block                []string       `mapstructure:"blocklist"`
	MaxSeriesPerRule     int            `mapstructure:"max_series_per_rule"`
	LabelSamplingEnabled bool           `mapstructure:"label_sampling_enabled"`
	LabelSamplingRate    float64        `mapstructure:"label_sampling_rate"`
	Extra                map[string]any `mapstructure:",omitempty"`
}

// NotifyConfig governs notification behavior and retries.
type NotifyConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Timeout time.Duration `mapstructure:"timeout"`
	Retry   RetryConfig   `mapstructure:"retry"`
}

// RetryConfig tunes retry logic for notifications.
type RetryConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	MaxAttempts  int           `mapstructure:"max_attempts"`
	InitialDelay time.Duration `mapstructure:"initial_delay"`
	MaxDelay     time.Duration `mapstructure:"max_delay"`
	Multiplier   float64       `mapstructure:"multiplier"`
}

// MemoryConfig configures adaptive memory usage to avoid OOMs and handle pressure.
type MemoryConfig struct {
	MaxMemoryBytes               int64         `mapstructure:"max_memory_bytes"`
	MaxMemoryPercent             float64       `mapstructure:"max_memory_percent"`
	MaxTraceEntries              int           `mapstructure:"max_trace_entries"`
	MaxLogEntries                int           `mapstructure:"max_log_entries"`
	MaxMetricEntries             int           `mapstructure:"max_metric_entries"`
	EnableAdaptiveScaling        bool          `mapstructure:"enable_adaptive_scaling"`
	ScaleUpThreshold             float64       `mapstructure:"scale_up_threshold"`
	ScaleDownThreshold           float64       `mapstructure:"scale_down_threshold"`
	ScaleCheckInterval           time.Duration `mapstructure:"scale_check_interval"`
	MaxScaleFactor               float64       `mapstructure:"max_scale_factor"`
	EnableMemoryPressureHandling bool          `mapstructure:"enable_memory_pressure_handling"`
	MemoryPressureThreshold      float64       `mapstructure:"memory_pressure_threshold"`
	SamplingRateUnderPressure    float64       `mapstructure:"sampling_rate_under_pressure"`
	UseRingBuffers               bool          `mapstructure:"use_ring_buffers"`
	RingBufferOverwrite          bool          `mapstructure:"ring_buffer_overwrite"`
}

// RuleCfg represents a single rule to evaluate.
type RuleCfg struct {
	Name        string            `mapstructure:"name"`
	Signal      string            `mapstructure:"signal"`
	Expr        ExprCfg           `mapstructure:"expr"`
	Window      time.Duration     `mapstructure:"window"`
	Step        time.Duration     `mapstructure:"step"`
	For         time.Duration     `mapstructure:"for"`
	GroupBy     []string          `mapstructure:"group_by"`
	Select      map[string]string `mapstructure:"select"`
	Severity    string            `mapstructure:"severity"`
	Enabled     bool              `mapstructure:"enabled"`
	Labels      map[string]string `mapstructure:"labels"`
	Description string            `mapstructure:"description"`
}

// ExprCfg describes the expression and comparison.
type ExprCfg struct {
	Type         string                 `mapstructure:"type"`
	Field        string                 `mapstructure:"field"`
	Op           string                 `mapstructure:"op"`
	Value        float64                `mapstructure:"value"`
	Quantile     float64                `mapstructure:"quantile"`
	RateDuration time.Duration          `mapstructure:"rate_duration"`
	Options      map[string]interface{} `mapstructure:"options"`
}

func CreateDefaultConfig() component.Config {
	return &Config{
		WindowSize: 5 * time.Second,
		Step:       5 * time.Second,
		InstanceID: "default",
		TSDB: &TSDBConfig{
			Enabled:                  false, // default: disabled so lifecycle tests don’t require TSDB
			QueryTimeout:             30 * time.Second,
			WriteTimeout:             10 * time.Second,
			DedupWindow:              30 * time.Second,
			EnableRemoteWrite:        true,
			RemoteWriteBatchSize:     1000,
			RemoteWriteFlushInterval: 5 * time.Second,
		},
		Dedup: DedupConfig{
			FingerprintLabels: []string{"alertname", "severity", "cluster", "namespace", "service"},
			ExcludeLabels:     []string{"pod_ip", "container_id", "timestamp", "trace_id"},
			Window:            30 * time.Second,
			EnableTSDBDedup:   true,
		},
		Limits: LimitsConfig{
			Storm: StormCfg{
				MaxTransitionsPerMinute:    100,
				MaxEventsPerInterval:       50,
				Interval:                   1 * time.Second,
				CircuitBreakerThreshold:    0.5,
				CircuitBreakerRecoveryTime: 30 * time.Second,
			},
			Cardinality: CardinalityCfg{
				MaxLabels:            20,
				MaxValLen:            128,
				MaxTotalSize:         2048,
				HashIfExceeds:        128,
				Allow:                []string{"alertname", "severity", "rule_id"},
				Block:                []string{"pod_ip", "container_id", "trace_id"},
				MaxSeriesPerRule:     1000,
				LabelSamplingEnabled: false,
				LabelSamplingRate:    0.1,
			},
		},
		Notify: NotifyConfig{
			Enabled: true,
			Timeout: 10 * time.Second,
			Retry: RetryConfig{
				Enabled:      true,
				MaxAttempts:  3,
				InitialDelay: 1 * time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
			},
		},
		Memory: MemoryConfig{
			MaxMemoryPercent:             0.10,
			MaxTraceEntries:              0,
			MaxLogEntries:                0,
			MaxMetricEntries:             0,
			EnableAdaptiveScaling:        true,
			ScaleUpThreshold:             0.8,
			ScaleDownThreshold:           0.4,
			ScaleCheckInterval:           30 * time.Second,
			MaxScaleFactor:               10.0,
			EnableMemoryPressureHandling: true,
			MemoryPressureThreshold:      0.85,
			SamplingRateUnderPressure:    0.10,
			UseRingBuffers:               false,
			RingBufferOverwrite:          false,
		},
		// Default ON so alert events also appear as a metric series if a metrics pipeline is wired.
		EmitAlertMetrics: true,
	}
}

// Validate performs semantic validation and applies defaults.
func (c *Config) Validate() error {
	if c.WindowSize <= 0 {
		return errors.New("window must be > 0")
	}
	if c.Step <= 0 {
		c.Step = c.WindowSize
	}
	if c.InstanceID == "" {
		c.InstanceID = "default"
	}

	// TSDB is OPTIONAL unless explicitly enabled.
	if c.TSDB != nil && c.TSDB.Enabled {
		if err := c.TSDB.validate(); err != nil {
			return err
		}
	}

	// Memory checks.
	if c.Memory.MaxMemoryPercent <= 0 || c.Memory.MaxMemoryPercent > 1.0 {
		return errors.New("memory.max_percent must be between 0 and 1")
	}
	if c.Memory.ScaleUpThreshold < 0 || c.Memory.ScaleUpThreshold > 1.0 {
		return errors.New("memory.scale_up_threshold must be between 0 and 1")
	}
	if c.Memory.ScaleDownThreshold < 0 || c.Memory.ScaleDownThreshold > 1.0 {
		return errors.New("memory.scale_down_threshold must be between 0 and 1")
	}
	if c.Memory.MemoryPressureThreshold < 0 || c.Memory.MemoryPressureThreshold > 1.0 {
		return errors.New("memory.memory_pressure_threshold must be between 0 and 1")
	}
	if c.Memory.SamplingRateUnderPressure < 0 || c.Memory.SamplingRateUnderPressure > 1.0 {
		return errors.New("memory.sampling_rate_under_pressure must be between 0 and 1")
	}

	// Rules validation.
	seen := map[string]struct{}{}
	for i := range c.Rules {
		r := &c.Rules[i]
		if err := r.validate(); err != nil {
			return err
		}
		if _, ok := seen[r.Name]; ok {
			return fmt.Errorf("duplicate rule name %q", r.Name)
		}
		seen[r.Name] = struct{}{}

		// Apply defaults
		if r.Severity == "" {
			r.Severity = "warning"
		}
		if !r.Enabled {
			// default is enabled unless explicitly set false; if zero-value bool, treat as enabled
			r.Enabled = true
		}
	}

	return nil
}

func (t *TSDBConfig) validate() error {
	if t.QueryURL == "" {
		return errors.New("tsdb.query_url required")
	}
	if t.RemoteWriteBatchSize < 0 {
		return errors.New("tsdb.remote_write_batch_size must be >= 0")
	}
	if t.RemoteWriteFlushInterval < 0 {
		return errors.New("tsdb.remote_write_flush_interval must be >= 0")
	}
	return nil
}

func (r *RuleCfg) validate() error {
	if r.Name == "" {
		return errors.New("name required")
	}
	switch r.Signal {
	case "metrics", "logs", "traces":
	default:
		return errors.New("invalid signal")
	}
	if err := r.Expr.validate(); err != nil {
		return err
	}
	return nil
}

func (e *ExprCfg) validate() error {
	if e.Type == "" {
		return errors.New("expr.type required")
	}
	if e.Op == "" {
		return errors.New("expr.op required")
	}
	switch e.Op {
	case ">", ">=", "<", "<=", "==", "!=":
	default:
		return fmt.Errorf("invalid expr.op %q", e.Op)
	}
	if e.Type == "quantile_over_time" && (e.Quantile < 0 || e.Quantile > 1) {
		return errors.New("expr.quantile must be within [0,1]")
	}
	if e.Type == "rate" && e.RateDuration <= 0 {
		return errors.New("expr.rate_duration must be > 0 for rate")
	}
	return nil
}
