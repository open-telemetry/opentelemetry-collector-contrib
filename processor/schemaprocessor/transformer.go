// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

type transformer struct {
	targets []string
	log     *zap.Logger
}

func newTransformer(
	_ context.Context,
	conf component.Config,
	set processor.CreateSettings,
) (*transformer, error) {
	cfg, ok := conf.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration provided")
	}
	return &transformer{
		log:     set.Logger,
		targets: cfg.Targets,
	}, nil
}

func (t transformer) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	return ld, nil
}

func (t transformer) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	return md, nil
}

func (t transformer) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	return td, nil
}

// start will load the remote file definition if it isn't already cached
// and resolve the schema translation file
func (t *transformer) start(_ context.Context, _ component.Host) error {
	for _, target := range t.targets {
		t.log.Info("Fetching remote schema url", zap.String("schema-url", target))
	}
	return nil
}
