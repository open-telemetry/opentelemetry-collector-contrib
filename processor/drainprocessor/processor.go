// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	internaldrain "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/drain"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

type drainProcessor struct {
	config      *Config
	componentID component.ID
	logger      *zap.Logger
	telemetry   *metadata.TelemetryBuilder

	mu       sync.Mutex
	drain    *internaldrain.Drain
	warmedUp bool // true when WarmupMinClusters == 0 or cluster count has reached the threshold

	storageClient    storage.Client
	stopSave         context.CancelFunc // cancels periodic save goroutine
	lastSnapshotHash atomic.Uint64
}

func newDrainProcessor(set processor.Settings, cfg *Config) (*drainProcessor, error) {
	d, err := internaldrain.NewDrain(internaldrain.Config{
		Depth:           cfg.TreeDepth,
		SimThreshold:    cfg.MergeThreshold,
		MaxChildren:     cfg.MaxNodeChildren,
		MaxClusters:     cfg.MaxClusters,
		ExtraDelimiters: cfg.ExtraDelimiters,
	})
	if err != nil {
		return nil, err
	}

	tel, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	p := &drainProcessor{
		config:      cfg,
		componentID: set.ID,
		logger:      set.Logger,
		telemetry:   tel,
		drain:       d,
		warmedUp:    cfg.WarmupMinClusters == 0,
	}
	return p, nil
}

// seed pre-populates the Drain tree from SeedTemplates and SeedLogs before any
// live log records arrive. Empty entries are skipped. Train failures are logged
// as warnings and skipped rather than aborting startup.
func (p *drainProcessor) seed() {
	for _, tmpl := range p.config.SeedTemplates {
		if strings.TrimSpace(tmpl) == "" {
			continue
		}
		if _, err := p.drain.Train(tmpl); err != nil {
			p.logger.Warn("failed to seed template, skipping", zap.String("template", tmpl), zap.Error(err))
		}
	}
	for _, line := range p.config.SeedLogs {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, err := p.drain.Train(line); err != nil {
			p.logger.Warn("failed to seed log line, skipping", zap.String("line", line), zap.Error(err))
		}
	}
}

// Start loads a snapshot from storage (if available) and starts the periodic
// save goroutine when configured.
func (p *drainProcessor) Start(ctx context.Context, host component.Host) error {
	if p.config.Storage != nil {
		var err error
		p.storageClient, err = getStorageClient(ctx, host, p.config.Storage, p.componentID)
		if err != nil {
			return fmt.Errorf("failed to get storage client: %w", err)
		}

		if !p.loadSnapshot(ctx) {
			p.seed()
		}

		if p.config.SaveInterval > 0 {
			p.startPeriodicSave(ctx)
		}
		return nil
	}

	p.seed()
	return nil
}

// Shutdown stops the periodic save goroutine, performs a final snapshot save,
// and closes the storage client.
func (p *drainProcessor) Shutdown(ctx context.Context) error {
	if p.stopSave != nil {
		p.stopSave()
	}

	var errs []error
	if p.storageClient != nil {
		if err := p.saveSnapshot(ctx); err != nil {
			p.logger.Warn("final snapshot save failed", zap.Error(err))
			errs = append(errs, err)
		}
		if err := p.storageClient.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// processLogs is the ConsumeLogs handler passed to processorhelper.NewLogs.
func (p *drainProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				p.annotate(ctx, lrs.At(k))
			}
		}
	}

	p.mu.Lock()
	count := p.drain.ClusterCount()
	p.mu.Unlock()
	p.telemetry.ProcessorDrainClustersActive.Record(ctx, int64(count))

	return ld, nil
}

func (p *drainProcessor) annotate(ctx context.Context, lr plog.LogRecord) {
	text := extractBody(lr, p.config.BodyField)
	if text == "" {
		return
	}

	p.mu.Lock()
	tmpl, err := p.drain.Train(text)
	if !p.warmedUp && p.drain.ClusterCount() >= p.config.WarmupMinClusters {
		p.warmedUp = true
	}
	warmedUp := p.warmedUp
	p.mu.Unlock()

	if err != nil {
		p.logger.Warn("drain Train failed, skipping annotation", zap.Error(err))
		return
	}
	if tmpl == "" || !warmedUp {
		return
	}

	lr.Attributes().PutStr(p.config.TemplateAttribute, tmpl)
	p.telemetry.ProcessorDrainLogRecordsAnnotated.Add(ctx, 1)
}

// extractBody returns the text to feed to Drain for the given log record.
// If bodyField is non-empty and the body is a map, the named field is extracted.
// Falls back to the full body string representation in all other cases.
func extractBody(lr plog.LogRecord, bodyField string) string {
	body := lr.Body()
	if bodyField != "" && body.Type() == pcommon.ValueTypeMap {
		if v, ok := body.Map().Get(bodyField); ok {
			return v.AsString()
		}
	}
	return body.AsString()
}
