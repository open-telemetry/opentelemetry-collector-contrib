// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	internaldrain "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/drain"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

type drainProcessor struct {
	config    *Config
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder

	mu    sync.Mutex
	drain *internaldrain.Drain

	// warmup state — only used when WarmupMode is "buffer"
	warmedUp      bool
	buffer        []plog.Logs
	bufferedCount int
}

func newDrainProcessor(set processor.Settings, cfg *Config) (*drainProcessor, error) {
	d, err := internaldrain.New(internaldrain.Config{
		Depth:           cfg.LogClusterDepth,
		SimThreshold:    cfg.SimThreshold,
		MaxChildren:     cfg.MaxChildren,
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
		config:    cfg,
		logger:    set.Logger,
		telemetry: tel,
		drain:     d,
	}
	p.seed()
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

// processLogs is the ConsumeLogs handler passed to processorhelper.NewLogs.
func (p *drainProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	var out plog.Logs
	if p.config.WarmupMode == warmupModePassthrough {
		out = p.annotateAll(ctx, ld)
	} else {
		out = p.processBuffered(ctx, ld)
	}
	p.mu.Lock()
	count := p.drain.ClusterCount()
	p.mu.Unlock()
	p.telemetry.ProcessorDrainClustersActive.Record(ctx, int64(count))
	return out, nil
}

// processBuffered implements the "buffer" warmup mode. During warmup, batches
// are trained (tree updated) but held in memory without annotation. Once the
// flush condition is met, all buffered batches are merged, annotated with the
// now-stable templates, and returned as a single combined batch.
func (p *drainProcessor) processBuffered(ctx context.Context, ld plog.Logs) plog.Logs {
	// Fast-path: warmup already complete.
	p.mu.Lock()
	if p.warmedUp {
		p.mu.Unlock()
		return p.annotateAll(ctx, ld)
	}
	p.mu.Unlock()

	// Train all records in this batch to update the tree and grow cluster count.
	// (attributes are not set yet — we wait for the tree to stabilize)
	p.trainBatch(ld)

	// Update buffer state under lock, then decide whether to flush.
	p.mu.Lock()

	// Re-check warmedUp: another goroutine may have flushed between the
	// fast-path check above and this lock acquisition. If so, annotateAll
	// calls Train again on these records (they were already trained in
	// trainBatch above). Training the same line twice is harmless — it
	// reinforces the existing cluster without creating a duplicate.
	if p.warmedUp {
		p.mu.Unlock()
		return p.annotateAll(ctx, ld)
	}

	p.buffer = append(p.buffer, ld)
	p.bufferedCount += ld.LogRecordCount()

	shouldFlush := p.drain.ClusterCount() >= p.config.WarmupMinClusters ||
		p.bufferedCount >= p.config.WarmupBufferMaxLogs

	if !shouldFlush {
		p.mu.Unlock()
		return plog.NewLogs()
	}

	// Warmup complete — take ownership of the buffer and flip the flag.
	toFlush := p.buffer
	p.buffer = nil
	p.bufferedCount = 0
	p.warmedUp = true
	p.mu.Unlock()

	// Merge all buffered batches into one, annotating as we go.
	// Re-annotating (rather than reusing the training pass) ensures records
	// get the final abstracted templates from the fully-warmed tree.
	merged := plog.NewLogs()
	for _, batch := range toFlush {
		p.annotateAll(ctx, batch) // annotate in-place
		batch.ResourceLogs().MoveAndAppendTo(merged.ResourceLogs())
	}
	return merged
}

// trainBatch calls Train on every record in ld without setting any attributes.
// Used during buffer warmup to grow the Drain tree without committing templates.
func (p *drainProcessor) trainBatch(ld plog.Logs) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				text := extractBody(lrs.At(k), p.config.BodyField)
				if text == "" {
					continue
				}
				p.mu.Lock()
				_, err := p.drain.Train(text)
				p.mu.Unlock()
				if err != nil {
					p.logger.Warn("drain Train failed during warmup buffering, skipping", zap.Error(err))
				}
			}
		}
	}
}

// annotateAll annotates every record in ld in-place and returns ld.
func (p *drainProcessor) annotateAll(ctx context.Context, ld plog.Logs) plog.Logs {
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
	return ld
}

func (p *drainProcessor) annotate(ctx context.Context, lr plog.LogRecord) {
	text := extractBody(lr, p.config.BodyField)
	if text == "" {
		p.telemetry.ProcessorDrainLogRecordsUnannotated.Add(ctx, 1)
		return
	}

	p.mu.Lock()
	tmpl, err := p.drain.Train(text)
	p.mu.Unlock()

	if err != nil {
		p.logger.Warn("drain Train failed, skipping annotation", zap.Error(err))
		p.telemetry.ProcessorDrainLogRecordsUnannotated.Add(ctx, 1)
		return
	}
	if tmpl == "" {
		// go-drain3 returned no cluster; skip annotation.
		p.telemetry.ProcessorDrainLogRecordsUnannotated.Add(ctx, 1)
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
