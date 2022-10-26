// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereloadereceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/multierr"
)

type runnerStatus int

const (
	statusCreated runnerStatus = iota
	statusStarted
	statusStartError
	statusStopped
	statusStopError
)

var _ component.MetricsReceiver = (*runner)(nil)
var _ component.LogsReceiver = (*runner)(nil)

type runner struct {
	receivers builtReceivers
	pipelines map[config.ComponentID]*pipeline
	cfghash   uint64
	path      string
	status    runnerStatus
}

func newRunner(ctx context.Context, path string, cfg *runnerConfig, settings component.TelemetrySettings, buildInfo component.BuildInfo, host component.Host, next baseConsumer, cType config.Type) (*runner, error) {
	pipelines, err := buildPipelines(ctx, settings, buildInfo, host, cfg, next)
	if err != nil {
		return nil, err
	}

	receivers, err := buildReceivers(ctx, settings, buildInfo, host, cfg, pipelines, cType)
	if err != nil {
		return nil, err
	}

	return &runner{
		receivers: receivers,
		pipelines: pipelines,
		path:      path,
		cfghash:   cfg.Hash(),
		status:    statusCreated,
	}, nil
}

func (r *runner) Start(ctx context.Context, host component.Host) error {
	var errs error
	for _, p := range r.pipelines {
		errs = multierr.Append(errs, p.StartProcessors(ctx, host))
	}

	for _, rc := range r.receivers {
		errs = multierr.Append(errs, rc.Start(ctx, host))
	}

	if errs == nil {
		r.status = statusStarted
	} else {
		r.status = statusStartError
	}
	return errs
}

func (r *runner) Shutdown(ctx context.Context) error {
	var errs error
	for _, rc := range r.receivers {
		errs = multierr.Append(errs, rc.Shutdown(ctx))
	}

	for _, p := range r.pipelines {
		errs = multierr.Append(errs, p.StopProcessors(ctx))
	}

	if errs == nil {
		r.status = statusStopped
	} else {
		r.status = statusStopError
	}
	return errs
}
