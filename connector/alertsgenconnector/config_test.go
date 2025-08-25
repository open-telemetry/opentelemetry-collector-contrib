// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"strings"
	"testing"
	"time"
)

func newDefaultCfg(t *testing.T) *Config {
	t.Helper()
	c, ok := CreateDefaultConfig().(*Config)
	if !ok {
		t.Fatalf("CreateDefaultConfig did not return *Config")
	}
	return c
}

func TestDefaultConfig_Validate_OK_TSDBDisabled(t *testing.T) {
	cfg := newDefaultCfg(t)
	if cfg.TSDB == nil {
		t.Fatalf("default TSDB block should not be nil")
	}
	// Explicitly ensure it's disabled so tests are stable if defaults change later.
	cfg.TSDB.Enabled = false

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() on default (TSDB disabled) returned error: %v", err)
	}

	// Step should be defaulted to Window when <= 0 (and is already equal by default).
	if cfg.Step != cfg.WindowSize {
		t.Fatalf("expected Step to equal WindowSize by default, got Step=%s Window=%s", cfg.Step, cfg.WindowSize)
	}

	// InstanceID should default to "default" (per code).
	if cfg.InstanceID != "default" {
		t.Fatalf("expected InstanceID to default to 'default', got %q", cfg.InstanceID)
	}
}

func TestTSDBEnabledWithoutURL_Err(t *testing.T) {
	cfg := newDefaultCfg(t)
	cfg.TSDB.Enabled = true
	cfg.TSDB.QueryURL = "" // intentionally missing

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected error when tsdb.enabled=true but query_url is empty")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "tsdb.query_url") {
		t.Fatalf("unexpected error: %v (want mention of tsdb.query_url required)", err)
	}
}

func TestTSDBEnabledWithURL_OK(t *testing.T) {
	cfg := newDefaultCfg(t)
	cfg.TSDB.Enabled = true
	cfg.TSDB.QueryURL = "http://prometheus:9090"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with TSDB enabled & URL set returned error: %v", err)
	}
}

func TestTSDBBatchingFields_Validation(t *testing.T) {
	// negative batch size -> error
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = true
		cfg.TSDB.QueryURL = "http://prometheus:9090"
		cfg.TSDB.RemoteWriteBatchSize = -1

		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for negative tsdb.remote_write_batch_size")
		}
	}

	// negative flush interval -> error
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = true
		cfg.TSDB.QueryURL = "http://prometheus:9090"
		cfg.TSDB.RemoteWriteFlushInterval = -1

		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for negative tsdb.remote_write_flush_interval")
		}
	}
}

func TestMemoryBounds_Validation(t *testing.T) {
	// MaxMemoryPercent <= 0
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Memory.MaxMemoryPercent = 0
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for memory.max_percent <= 0")
		}
	}

	// MaxMemoryPercent > 1
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Memory.MaxMemoryPercent = 1.01
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for memory.max_percent > 1")
		}
	}

	// Scale thresholds out of bounds
	for name, mutate := range map[string]func(*Config){
		"scale_up_threshold > 1":           func(c *Config) { c.Memory.ScaleUpThreshold = 1.1 },
		"scale_down_threshold < 0":         func(c *Config) { c.Memory.ScaleDownThreshold = -0.1 },
		"memory_pressure_threshold > 1":    func(c *Config) { c.Memory.MemoryPressureThreshold = 1.1 },
		"sampling_rate_under_pressure < 0": func(c *Config) { c.Memory.SamplingRateUnderPressure = -0.1 },
		"sampling_rate_under_pressure > 1": func(c *Config) { c.Memory.SamplingRateUnderPressure = 1.1 },
	} {
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		mutate(cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for %s", name)
		}
	}
}

func TestRulesValidation_AndDefaults(t *testing.T) {
	// Missing name -> error
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "",
				Signal: "metrics",
				Expr: ExprCfg{
					Type:  "rate",
					Field: "value",
					Op:    ">",
					Value: 1,
					// for "rate", RateDuration must be > 0 — leave zero to check expr path separately later
					RateDuration: time.Second,
				},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for missing rule name")
		}
	}

	// Invalid signal -> error
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "r1",
				Signal: "bad-signal",
				Expr: ExprCfg{
					Type:         "rate",
					Field:        "value",
					Op:           ">",
					Value:        1,
					RateDuration: time.Second,
				},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for invalid signal")
		}
	}

	// Duplicate rule names -> error
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "dup",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "rate", Field: "v", Op: ">", Value: 1, RateDuration: time.Second},
			},
			{
				Name:   "dup",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "rate", Field: "v", Op: ">", Value: 2, RateDuration: time.Second},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for duplicate rule names")
		}
	}

	// Defaults applied: severity -> "warning", enabled -> true when zero value
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "ok",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "rate", Field: "v", Op: ">", Value: 1, RateDuration: time.Second},
				// Severity empty -> should default to "warning"
				// Enabled zero-value (false) -> should be set to true by Validate
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("unexpected validate error: %v", err)
		}
		if got := cfg.Rules[0].Severity; got != "warning" {
			t.Fatalf("expected default severity 'warning', got %q", got)
		}
		if got := cfg.Rules[0].Enabled; !got {
			t.Fatalf("expected Enabled to default to true, got false")
		}
	}
}

func TestExprValidation(t *testing.T) {
	// Missing type
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "r",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "", Field: "v", Op: ">", Value: 1},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for missing expr.type")
		}
	}

	// Missing op
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "r",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "rate", Field: "v", Op: "", Value: 1, RateDuration: time.Second},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for missing expr.op")
		}
	}

	// Invalid op
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "r",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "rate", Field: "v", Op: "!!", Value: 1, RateDuration: time.Second},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for invalid expr.op")
		}
	}

	// quantile_over_time: quantile out of range
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "q",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "quantile_over_time", Field: "latency", Op: ">", Value: 1, Quantile: 1.5},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for quantile not within [0,1]")
		}
	}

	// rate: RateDuration must be > 0
	{
		cfg := newDefaultCfg(t)
		cfg.TSDB.Enabled = false
		cfg.Rules = []RuleCfg{
			{
				Name:   "rate",
				Signal: "metrics",
				Expr:   ExprCfg{Type: "rate", Field: "v", Op: ">", Value: 1, RateDuration: 0},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected error for rate with non-positive rate_duration")
		}
	}
}

func TestStepDefaultsToWindowWhenNonPositive(t *testing.T) {
	cfg := newDefaultCfg(t)
	cfg.TSDB.Enabled = false
	cfg.Step = 0
	cfg.WindowSize = 2 * time.Second

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}
	if cfg.Step != cfg.WindowSize {
		t.Fatalf("expected Step defaulted to WindowSize; got Step=%s Window=%s", cfg.Step, cfg.WindowSize)
	}
}

func TestTSDBOptional_WhenDisabled_NoErrorEvenIfURLMissing(t *testing.T) {
	cfg := newDefaultCfg(t)
	cfg.TSDB.Enabled = false
	cfg.TSDB.QueryURL = "" // missing is fine because TSDB is disabled

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error with tsdb disabled: %v", err)
	}
}
