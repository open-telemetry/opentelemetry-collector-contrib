// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	internaldrain "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/drain"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

// compiledMaskRule is a MaskingRule with its pattern pre-compiled and its
// replacement token materialized.
type compiledMaskRule struct {
	name    string
	token   string // "<name>"
	pattern *regexp.Regexp
}

type drainProcessor struct {
	config      *Config
	componentID component.ID
	logger      *zap.Logger
	telemetry   *metadata.TelemetryBuilder

	mu       sync.Mutex
	drain    *internaldrain.Drain
	warmedUp bool // true when WarmupMinClusters == 0 or cluster count has reached the threshold

	// masks holds compiled masking rules in declaration order.
	masks []compiledMaskRule
	// maskTokenNames maps a mask token (e.g. "<ip>") back to its bare name
	// (e.g. "ip"). Used during parameter extraction to name variable positions
	// in the template. nil when no rules are configured.
	maskTokenNames map[string]string

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

	masks, tokenNames, err := compileMaskingRules(cfg.MaskingRules)
	if err != nil {
		return nil, err
	}

	tel, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	p := &drainProcessor{
		config:         cfg,
		componentID:    set.ID,
		logger:         set.Logger,
		telemetry:      tel,
		drain:          d,
		warmedUp:       cfg.WarmupMinClusters == 0,
		masks:          masks,
		maskTokenNames: tokenNames,
	}
	return p, nil
}

// compileMaskingRules pre-compiles the configured rules. Validate is expected
// to have already caught invalid regex; we recompile here so the processor
// holds compiled forms without leaning on Validate side-effects.
func compileMaskingRules(rules []MaskingRule) ([]compiledMaskRule, map[string]string, error) {
	if len(rules) == 0 {
		return nil, nil, nil
	}
	compiled := make([]compiledMaskRule, 0, len(rules))
	tokenNames := make(map[string]string, len(rules))
	for i, r := range rules {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("masking_rules[%d]: compile %q: %w", i, r.Pattern, err)
		}
		token := "<" + r.Name + ">"
		compiled = append(compiled, compiledMaskRule{name: r.Name, token: token, pattern: re})
		tokenNames[token] = r.Name
	}
	return compiled, tokenNames, nil
}

// applyMasks returns text with each configured masking rule applied in order.
// Empty input and no-rule configurations short-circuit.
func (p *drainProcessor) applyMasks(text string) string {
	if len(p.masks) == 0 || text == "" {
		return text
	}
	for _, r := range p.masks {
		text = r.pattern.ReplaceAllLiteralString(text, r.token)
	}
	return text
}

// seed pre-populates the Drain tree from SeedTemplates and SeedLogs before any
// live log records arrive. Empty entries are skipped. Train failures are logged
// as warnings and skipped rather than aborting startup.
//
// SeedTemplates are trained verbatim — the user is declaring template shape,
// so mask tokens they include appear as-is. SeedLogs go through the same
// masking pass as live records so the tree learns the same shape.
func (p *drainProcessor) seed() {
	for _, tmpl := range p.config.SeedTemplates {
		if strings.TrimSpace(tmpl) == "" {
			continue
		}
		if _, _, err := p.drain.Train(tmpl); err != nil {
			p.logger.Warn("failed to seed template, skipping", zap.String("template", tmpl), zap.Error(err))
		}
	}
	for _, line := range p.config.SeedLogs {
		if strings.TrimSpace(line) == "" {
			continue
		}
		masked := p.applyMasks(line)
		if _, _, err := p.drain.Train(masked); err != nil {
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
	raw := extractBody(lr, p.config.BodyField)
	if raw == "" {
		return
	}
	masked := p.applyMasks(raw)

	p.mu.Lock()
	tmpl, tmplTokens, err := p.drain.Train(masked)
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
	if len(p.masks) > 0 || p.config.EmitWildcards {
		p.extractParams(ctx, lr, raw, tmplTokens)
	}
	p.telemetry.ProcessorDrainLogRecordsAnnotated.Add(ctx, 1)
}

// extractParams writes extracted parameters to attributes on lr:
//
//   - Each mask-matched position writes a dynamic attribute keyed as
//     "<ParameterKeyPrefix>.<mask name>" with the raw body value. First-match
//     wins on duplicate mask names; losing values are dropped and each
//     duplicated name increments the masks-duplicates metric once per record.
//   - When EmitWildcards is true, Drain's own "<*>" positions are written as
//     a positional string slice attribute at WildcardsAttribute in template
//     order.
//
// Extraction relies on positional alignment between raw body tokens and
// template tokens. When a mask pattern spans whitespace, the masked line has
// fewer tokens than the raw line, alignment breaks, and this function skips
// extraction rather than emit misaligned values. The template attribute is
// still written by the caller.
func (p *drainProcessor) extractParams(ctx context.Context, lr plog.LogRecord, rawBody string, tmplTokens []string) {
	bodyTokens := p.drain.Tokenise(rawBody)
	if len(bodyTokens) == 0 || len(bodyTokens) != len(tmplTokens) {
		return
	}

	attrs := lr.Attributes()
	var wildcards []string       // deferred allocation; only touched when EmitWildcards
	var seenMasks map[string]int // occurrences per mask name this record; nil until needed

	for i, t := range tmplTokens {
		if t == "<*>" {
			if p.config.EmitWildcards {
				wildcards = append(wildcards, bodyTokens[i])
			}
			continue
		}
		name, ok := p.maskTokenNames[t]
		if !ok {
			continue
		}
		if seenMasks == nil {
			seenMasks = make(map[string]int, len(p.masks))
		}
		seenMasks[name]++
		switch seenMasks[name] {
		case 1:
			attrs.PutStr(p.config.ParameterKeyPrefix+"."+name, bodyTokens[i])
		case 2:
			// First-match wins: an earlier position already claimed this
			// attribute key. Record the collision once per record per mask
			// name, regardless of how many further positions collide.
			p.telemetry.ProcessorDrainMasksDuplicates.Add(ctx, 1,
				metric.WithAttributes(attribute.String("mask", name)))
		}
	}

	if p.config.EmitWildcards && len(wildcards) > 0 {
		slice := attrs.PutEmptySlice(p.config.WildcardsAttribute)
		slice.EnsureCapacity(len(wildcards))
		for _, v := range wildcards {
			slice.AppendEmpty().SetStr(v)
		}
	}
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
