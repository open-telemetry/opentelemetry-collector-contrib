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
	"fmt"
	"time"

	"go.opencensus.io/metric/metricdata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

type reloader struct {
	conf        *Config
	settings    component.ReceiverCreateSettings
	next        baseConsumer
	watcher     *fileWatcher
	runners     map[uint64]*runner
	cType       config.Type
	logger      *zap.Logger
	instruments instruments
}

func newReloader(conf *Config, set component.ReceiverCreateSettings, next baseConsumer, dataType config.DataType) (component.Receiver, error) {
	if next == nil {
		return nil, component.ErrNilNextConsumer
	}

	return &reloader{
		conf:     conf,
		settings: set,
		next:     next,
		cType:    dataType,
		runners:  make(map[uint64]*runner),
		logger: set.Logger.With(
			zap.String(ZapNameKey, typeStr),
			zap.String(ZapKindKey, string(config.MetricsDataType)),
		),
		instruments: *globalInstruments,
	}, nil
}

func (m *reloader) Start(ctx context.Context, host component.Host) error {
	metricPipelineLabel := metricdata.NewLabelValue(string(m.cType))

	err := m.instruments.activePipelines.UpsertEntry(func() int64 {
		return int64(len(m.runners))
	}, metricPipelineLabel)
	if err != nil {
		return fmt.Errorf("failed to create active pipeline metric: %v", err)
	}

	// receiver metrics
	succeededLabel := metricdata.NewLabelValue(succeeded)
	failedLabel := metricdata.NewLabelValue(failed)
	createConfigErrorLabel := metricdata.NewLabelValue("create_config_error")
	newRunnerErrorLabel := metricdata.NewLabelValue("create_config_error")

	fileChangesEntry, _ := m.instruments.fileChangesTotal.GetEntry()
	failedReloadsEntry, _ := m.instruments.reloadsTotal.GetEntry(failedLabel)
	succeededReloadsEntry, _ := m.instruments.reloadsTotal.GetEntry(succeededLabel)
	createConfigErrorsEntry, _ := m.instruments.createPipelineErrorsTotal.GetEntry(metricPipelineLabel, createConfigErrorLabel)
	newRunnerErrorsEntry, _ := m.instruments.createPipelineErrorsTotal.GetEntry(metricPipelineLabel, newRunnerErrorLabel)
	succeededStartedPipelinesEntry, _ := m.instruments.startedPipelinesTotal.GetEntry(metricPipelineLabel, succeededLabel)
	failedStartedPipelinesEntry, _ := m.instruments.startedPipelinesTotal.GetEntry(metricPipelineLabel, failedLabel)
	succeededStoppedPipelinesEntry, _ := m.instruments.stoppedPipelinesTotal.GetEntry(metricPipelineLabel, succeededLabel)
	failedStoppedPipelinesEntry, _ := m.instruments.stoppedPipelinesTotal.GetEntry(metricPipelineLabel, failedLabel)

	m.watcher = newFileWatcher(m.conf.Path)

	go func() {
		t := time.NewTicker(m.conf.Reload.Period)

		for {
			select {
			case <-ctx.Done():
				m.logger.Debug("reload stopped")
				return
			case <-t.C:
				// List changes to reload
				// for each file that is detected do a stop/start
				m.logger.Debug("start reload")
				paths, changed, err := m.watcher.List()

				// Skip if we were unable to properly process the config directory
				if err != nil {
					failedReloadsEntry.Inc(1)
					m.logger.Warn("failed to list files", zap.Error(err))
					continue
				}

				succeededReloadsEntry.Inc(1)
				if !changed {
					continue
				}

				fileChangesEntry.Inc(1)
				stopList := make(map[uint64]*runner)
				for k, v := range m.runners {
					stopList[k] = v
				}

				startList := make([]*runnerConfig, 0)

				for _, path := range paths {
					cfg, err := newRunnerConfigFromFile(ctx, path, m.cType)
					if err != nil {
						createConfigErrorsEntry.Inc(1)
						m.logger.Warn("failed to create runner config", zap.String("path", path), zap.Error(err))
						continue
					}

					if _, ok := m.runners[cfg.Hash()]; ok {
						delete(stopList, cfg.Hash())
					} else {
						startList = append(startList, cfg)
					}
				}

				for h, s := range stopList {
					err := s.Shutdown(ctx)
					if err != nil {
						m.logger.Warn("failed to stop runner", zap.String("path", s.path), zap.Error(err))
						failedStoppedPipelinesEntry.Inc(1)
					} else {
						delete(m.runners, h)
						m.logger.Info("stopped runner", zap.String("path", s.path), zap.Error(err))
						succeededStoppedPipelinesEntry.Inc(1)
					}
				}

				for _, s := range startList {
					r, err := newRunner(ctx, s.path, s, m.settings.TelemetrySettings, m.settings.BuildInfo, host, m.next, m.cType)
					if err != nil {
						newRunnerErrorsEntry.Inc(1)
						m.logger.Warn("failed to create runner", zap.String("path", s.path), zap.Error(err))
						continue
					}
					err = r.Start(ctx, host)
					if err != nil {
						m.logger.Warn("failed to start runner", zap.String("path", s.path), zap.Error(err))
						failedStartedPipelinesEntry.Inc(1)
					} else {
						hash := s.Hash()
						if runner, exist := m.runners[hash]; exist {
							// will this happen? just in case, add some logs
							m.logger.Warn("found duplicated runner", zap.String("runner", fmt.Sprintf("%+v", runner)), zap.String("path", s.path))
						}
						m.runners[hash] = r
						m.logger.Info("started runner", zap.String("path", s.path), zap.Error(err))
						succeededStartedPipelinesEntry.Inc(1)
					}
				}
			}
			m.logger.Debug("reload done")
		}
	}()
	return nil
}

func (m *reloader) Shutdown(ctx context.Context) error {
	// Stop all dynamic configs
	for _, r := range m.runners {
		_ = r.Shutdown(ctx)
	}

	// Cleanup active pipeline metrics reporting
	_ = m.instruments.activePipelines.UpsertEntry(func() int64 {
		return 0
	}, metricdata.NewLabelValue(string(m.cType)))

	return nil
}
