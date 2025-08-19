// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"time"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	eval "github.com/platformbuilds/opentelemetry-collector-contrib/connector/alertsgenconnector/internal/eval"
	state "github.com/platformbuilds/opentelemetry-collector-contrib/connector/alertsgenconnector/internal/state"
	storm "github.com/platformbuilds/opentelemetry-collector-contrib/connector/alertsgenconnector/internal/storm"
)

type core struct { cfg *Config; log *zap.Logger; eng eval.Engine; store *state.Store; gov *storm.Governor; start time.Time; activeByKey map[string]int64; sinceByFP map[string]time.Time }

func newCore(set component.TelemetrySettings, cfg *Config) (*core, error) { eng, err := eval.NewFromFile(cfg.RulesFile); if err != nil { return nil, err } ; return &core{ cfg:cfg, log:set.Logger, eng:eng, store:state.New(cfg, set.Logger), gov:storm.NewGovernor(cfg.GovernorRPS, cfg.GovernorBurst, set.Logger), activeByKey: map[string]int64{}, sinceByFP: map[string]time.Time{} }, nil }
