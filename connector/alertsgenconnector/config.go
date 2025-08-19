// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"fmt"
	"time"
	"go.opentelemetry.io/collector/component"
)

type SeverityMap struct {
	Info []string `mapstructure:"info"`
	Warning []string `mapstructure:"warning"`
	Error []string `mapstructure:"error"`
	Fatal []string `mapstructure:"fatal"`
}

type Route struct { ToLogs bool `mapstructure:"to_logs"`; ToMetrics bool `mapstructure:"to_metrics"` }

type Config struct { component.Config
	RulesFile string `mapstructure:"rules_file"`
	MaxCardinality int `mapstructure:"max_cardinality"`
	ForDuration time.Duration `mapstructure:"for"`
	GracePeriod time.Duration `mapstructure:"grace_period"`
	GovernorRPS int `mapstructure:"governor_rps"`
	GovernorBurst int `mapstructure:"governor_burst"`
	Severity SeverityMap `mapstructure:"severity_map"`
	Routes Route `mapstructure:"routes"`
	CarryResource bool `mapstructure:"carry_resource"`
	IncludeSpanAttrs []string `mapstructure:"include_span_attributes"` }

func (c *Config) Validate() error {
	if c.GovernorRPS < 0 || c.GovernorBurst < 0 { return fmt.Errorf("governor rps/burst must be >= 0") }
	if c.ForDuration < 0 || c.GracePeriod < 0 { return fmt.Errorf("'for' and 'grace_period' must be >= 0") }
	return nil }

func createDefaultConfig() component.Config { return &Config{ MaxCardinality:10000, ForDuration:time.Second, GracePeriod:0, GovernorRPS:1000, GovernorBurst:2000, Routes:Route{ToLogs:true, ToMetrics:true}, CarryResource:true, IncludeSpanAttrs:[]string{"http.method","rpc.system","db.system"} } }
