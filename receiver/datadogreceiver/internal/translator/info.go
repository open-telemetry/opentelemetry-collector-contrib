// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

type ReducedObfuscationConfig struct {
	ElasticSearch        bool                      `json:"elastic_search"`
	Mongo                bool                      `json:"mongo"`
	SQLExecPlan          bool                      `json:"sql_exec_plan"`
	SQLExecPlanNormalize bool                      `json:"sql_exec_plan_normalize"`
	HTTP                 obfuscate.HTTPConfig      `json:"http"`
	RemoveStackTraces    bool                      `json:"remove_stack_traces"`
	Redis                obfuscate.RedisConfig     `json:"redis"`
	Memcached            obfuscate.MemcachedConfig `json:"memcached"`
}

type ReducedConfig struct {
	DefaultEnv             string                        `json:"default_env"`
	TargetTPS              float64                       `json:"target_tps"`
	MaxEPS                 float64                       `json:"max_eps"`
	ReceiverPort           int                           `json:"receiver_port"`
	ReceiverSocket         string                        `json:"receiver_socket"`
	ConnectionLimit        int                           `json:"connection_limit"`
	ReceiverTimeout        int                           `json:"receiver_timeout"`
	MaxRequestBytes        int64                         `json:"max_request_bytes"`
	StatsdPort             int                           `json:"statsd_port"`
	MaxMemory              float64                       `json:"max_memory"`
	MaxCPU                 float64                       `json:"max_cpu"`
	AnalyzedSpansByService map[string]map[string]float64 `json:"analyzed_spans_by_service"`
	Obfuscation            ReducedObfuscationConfig      `json:"obfuscation"`
}

type DDInfo struct {
	Version          string         `json:"version"`
	Endpoints        []string       `json:"endpoints"`
	ClientDropP0s    bool           `json:"client_drop_p0s"`
	SpanMetaStructs  bool           `json:"span_meta_structs"`
	LongRunningSpans bool           `json:"long_running_spans"`
	Config           *ReducedConfig `json:"config"`
}
