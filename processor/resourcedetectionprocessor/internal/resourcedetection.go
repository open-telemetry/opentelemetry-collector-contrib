// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package internal contains an interface for detecting resource information,
// and a provider to merge the resources returned by a slice of custom detectors.
package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v5"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

type DetectorType string

type Detector interface {
	Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error)
}

type DetectorConfig any

type ResourceDetectorConfig interface {
	GetConfigFromType(DetectorType) DetectorConfig
}

type DetectorFactory func(processor.Settings, DetectorConfig, bool) (Detector, error)

// detectorEntry pairs a detector with its type so detection telemetry can be
// attributed to the specific detector.
type detectorEntry struct {
	detectorType DetectorType
	detector     Detector
}

type ResourceProviderFactory struct {
	// detectors holds all possible detector types.
	detectors map[DetectorType]DetectorFactory
}

func NewProviderFactory(detectors map[DetectorType]DetectorFactory) *ResourceProviderFactory {
	return &ResourceProviderFactory{detectors: detectors}
}

func (f *ResourceProviderFactory) CreateResourceProvider(
	params processor.Settings,
	backoffConfig configretry.BackOffConfig,
	failOnMissingMetadata bool,
	detectorConfigs ResourceDetectorConfig,
	detectorTypes ...DetectorType,
) (*ResourceProvider, error) {
	detectors, err := f.getDetectors(params, detectorConfigs, detectorTypes, failOnMissingMetadata)
	if err != nil {
		return nil, err
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	provider := NewResourceProvider(params.Logger, telemetryBuilder, backoffConfig, detectors...)

	// Register observer for the detected-attribute count.
	if err := telemetryBuilder.RegisterResourcedetectionAttributesDetectedCallback(func(_ context.Context, o metric.Int64Observer) error {
		if r := provider.detectedResource.Load(); r != nil {
			o.Observe(int64(r.resource.Attributes().Len()))
		}
		return nil
	}); err != nil {
		telemetryBuilder.Shutdown()
		return nil, err
	}

	return provider, nil
}

func (f *ResourceProviderFactory) getDetectors(params processor.Settings, detectorConfigs ResourceDetectorConfig, detectorTypes []DetectorType, failOnMissingMetadata bool) ([]detectorEntry, error) {
	detectors := make([]detectorEntry, 0, len(detectorTypes))
	for _, detectorType := range detectorTypes {
		detectorFactory, ok := f.detectors[detectorType]
		if !ok {
			return nil, fmt.Errorf("invalid detector key: %v", detectorType)
		}

		detector, err := detectorFactory(params, detectorConfigs.GetConfigFromType(detectorType), failOnMissingMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed creating detector type %q: %w", detectorType, err)
		}

		detectors = append(detectors, detectorEntry{detectorType: detectorType, detector: detector})
	}

	return detectors, nil
}

type ResourceProvider struct {
	logger           *zap.Logger
	telemetry        *metadata.TelemetryBuilder
	backoffConfig    configretry.BackOffConfig
	detectors        []detectorEntry
	detectedResource atomic.Pointer[resourceResult]

	// Refresh loop control
	refreshInterval time.Duration
	stopCh          chan struct{}
	cancelFunc      context.CancelFunc
	wg              sync.WaitGroup
	startOnce       sync.Once
	stopOnce        sync.Once
}

type resourceResult struct {
	resource  pcommon.Resource
	schemaURL string
	err       error
}

func NewResourceProvider(logger *zap.Logger, telemetry *metadata.TelemetryBuilder, backoffConfig configretry.BackOffConfig, detectors ...detectorEntry) *ResourceProvider {
	return &ResourceProvider{
		logger:          logger,
		telemetry:       telemetry,
		backoffConfig:   backoffConfig,
		detectors:       detectors,
		refreshInterval: 0, // No periodic refresh by default
	}
}

func (p *ResourceProvider) Get(_ context.Context, _ *http.Client) (pcommon.Resource, string, error) {
	result := p.detectedResource.Load()
	if result != nil {
		return result.resource, result.schemaURL, result.err
	}
	return pcommon.NewResource(), "", nil
}

// Refresh recomputes the resource, replacing any previous result.
func (p *ResourceProvider) Refresh(ctx context.Context, client *http.Client) error {
	// MaxElapsedTime, when set, is the definitive bound on the whole retry
	// session: apply it regardless of client.Timeout, since backoff.Retry only
	// checks elapsed time between attempts, not during one — a per-attempt cap
	// larger than MaxElapsedTime would otherwise let a single attempt overrun
	// the budget. Otherwise fall back to client.Timeout so a single-shot or
	// unbounded-retry session still can't hang forever.
	switch {
	case p.backoffConfig.MaxElapsedTime > 0:
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.backoffConfig.MaxElapsedTime)
		defer cancel()
	case client.Timeout > 0:
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, client.Timeout)
		defer cancel()
	}

	res, schemaURL, err := p.detectResource(ctx, client)
	prev := p.detectedResource.Load()

	// Check if we have a previous successful snapshot
	hadPrevSuccess := prev != nil && prev.err == nil && !IsEmptyResource(prev.resource)

	// Keep the last good snapshot if the refresh errored.
	// Note: An empty resource with no error is considered a success (e.g., detector determined
	// it's not running on that cloud provider), so we accept it rather than keeping stale data.
	if hadPrevSuccess && err != nil {
		p.logger.Warn("resource refresh failed; keeping previous snapshot", zap.Error(err))
		// Return nil error since we're successfully keeping the cached resource
		return nil
	}

	// Accept the new snapshot (even if empty, as long as there was no error).
	p.detectedResource.Store(&resourceResult{
		resource:  res,
		schemaURL: schemaURL,
		err:       err,
	})

	return err
}

func (p *ResourceProvider) detectResource(ctx context.Context, client *http.Client) (pcommon.Resource, string, error) {
	res := pcommon.NewResource()
	mergedSchemaURL := ""
	var joinedErr error
	successes := 0

	p.logger.Info("began detecting resource information")

	resultsChan := make([]chan resourceResult, len(p.detectors))
	for i, entry := range p.detectors {
		ch := make(chan resourceResult, 1)
		resultsChan[i] = ch

		go func(entry detectorEntry, ch chan resourceResult) {
			p.detectWithRetry(ctx, client, entry, ch)
		}(entry, ch)
	}

	for _, ch := range resultsChan {
		result := <-ch
		if result.err != nil {
			joinedErr = errors.Join(joinedErr, result.err)
			continue
		}
		successes++
		mergedSchemaURL = MergeSchemaURL(mergedSchemaURL, result.schemaURL)
		MergeResource(res, result.resource, false)
	}

	p.logger.Info("detected resource information", zap.Any("resource", res.Attributes().AsRaw()))

	var returnErr error
	if successes == 0 && joinedErr == nil {
		returnErr = errors.New("resource detection failed: no detectors succeeded")
	} else {
		returnErr = joinedErr
	}

	// If all detectors failed, return empty resource.
	if successes == 0 {
		return pcommon.NewResource(), "", returnErr
	}

	// Partial or full success: return merged resources.
	return res, mergedSchemaURL, returnErr
}

func attemptContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

// detectWithRetry runs a detector with backoff. With Enabled=false it makes one
// attempt. With MaxElapsedTime > 0 each attempt is capped at client.Timeout so
// one hanging attempt can't eat the whole retry budget. Every attempt records
// per-detector result and duration telemetry.
func (p *ResourceProvider) detectWithRetry(ctx context.Context, client *http.Client, entry detectorEntry, ch chan resourceResult) {
	startTime := time.Now()
	detectorAttr := attribute.String("detector", string(entry.detectorType))

	// record emits the per-detector result and duration. On failure, failErr
	// classifies the error.type on the results counter.
	record := func(outcome string, failErr error) {
		base := []attribute.KeyValue{detectorAttr, attribute.String("outcome", outcome)}
		p.telemetry.ResourcedetectionDetectorDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(base...))
		if failErr != nil {
			base = append(base, semconv.ErrorType(failErr))
		}
		p.telemetry.ResourcedetectionDetectorResults.Add(ctx, 1, metric.WithAttributes(base...))
	}

	if !p.backoffConfig.Enabled {
		attemptCtx, cancel := attemptContext(ctx, client.Timeout)
		defer cancel()
		r, schemaURL, err := entry.detector.Detect(attemptCtx)
		if err != nil {
			p.logger.Warn("failed to detect resource", zap.String("detector", string(entry.detectorType)), zap.Error(err))
			record("failure", err)
			ch <- resourceResult{err: err}
			return
		}
		record("success", nil)
		ch <- resourceResult{resource: r, schemaURL: schemaURL}
		return
	}

	sleep := &backoff.ExponentialBackOff{
		InitialInterval:     p.backoffConfig.InitialInterval,
		RandomizationFactor: p.backoffConfig.RandomizationFactor,
		Multiplier:          p.backoffConfig.Multiplier,
		MaxInterval:         p.backoffConfig.MaxInterval,
	}

	opts := []backoff.RetryOption{
		backoff.WithBackOff(sleep),
	}
	// MaxElapsedTime == 0 disables the default 15-minute cap, enabling "retry forever".
	opts = append(opts, backoff.WithMaxElapsedTime(p.backoffConfig.MaxElapsedTime))

	type detectResult struct {
		resource  pcommon.Resource
		schemaURL string
	}

	perAttemptTimeout := time.Duration(0)
	if p.backoffConfig.MaxElapsedTime > 0 {
		perAttemptTimeout = client.Timeout
	}

	// Classify error.type from the first attempt's error: it's the deterministic
	// root cause, whereas the last attempt's error often just reflects the context
	// deadline firing rather than why the detector actually failed.
	var firstErr, lastDetErr error
	result, err := backoff.Retry(ctx, func() (detectResult, error) {
		attemptCtx, cancel := attemptContext(ctx, perAttemptTimeout)
		defer cancel()

		r, schemaURL, detErr := entry.detector.Detect(attemptCtx)
		if detErr != nil {
			lastDetErr = detErr
			if firstErr == nil {
				firstErr = detErr
			}
			p.logger.Warn("failed to detect resource, will retry", zap.String("detector", string(entry.detectorType)), zap.Error(detErr))
			return detectResult{}, detErr
		}
		return detectResult{resource: r, schemaURL: schemaURL}, nil
	}, opts...)
	if err != nil {
		// Preserve the underlying detector error so callers see the real cause,
		// not just bare context.DeadlineExceeded / backoff.PermanentError wrappers.
		if lastDetErr != nil && !errors.Is(err, lastDetErr) {
			err = errors.Join(lastDetErr, err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			p.logger.Warn("resource detection cancelled", zap.String("detector", string(entry.detectorType)), zap.Error(err))
		} else {
			p.logger.Error("resource detection retry budget exhausted", zap.String("detector", string(entry.detectorType)), zap.Error(err))
		}
		record("failure", firstErr)
		ch <- resourceResult{err: err}
		return
	}

	record("success", nil)
	ch <- resourceResult{resource: result.resource, schemaURL: result.schemaURL}
}

func MergeSchemaURL(currentSchemaURL, newSchemaURL string) string {
	if currentSchemaURL == "" {
		return newSchemaURL
	}
	if newSchemaURL == "" {
		return currentSchemaURL
	}
	if currentSchemaURL == newSchemaURL {
		return currentSchemaURL
	}
	// TODO: handle the case when the schema URLs are different by performing
	// schema conversion. For now we simply ignore the new schema URL.
	return currentSchemaURL
}

func MergeResource(to, from pcommon.Resource, overrideTo bool) {
	if IsEmptyResource(from) {
		return
	}

	toAttr := to.Attributes()
	fromAttr := from.Attributes()
	if toAttr.Len() == 0 {
		toAttr.EnsureCapacity(fromAttr.Len())
		fromAttr.CopyTo(toAttr)
		return
	}

	for k, v := range fromAttr.All() {
		if overrideTo {
			v.CopyTo(toAttr.PutEmpty(k))
		} else {
			if targetVal, found := toAttr.GetOrPutEmpty(k); !found {
				v.CopyTo(targetVal)
			}
		}
	}
}

func IsEmptyResource(res pcommon.Resource) bool {
	if res == (pcommon.Resource{}) {
		return true
	}
	return res.Attributes().Len() == 0
}

// StartRefreshing begins periodic resource refresh if refreshInterval > 0.
// It is safe to call multiple times; only the first call starts the goroutine.
func (p *ResourceProvider) StartRefreshing(refreshInterval time.Duration, client *http.Client) {
	p.startOnce.Do(func() {
		p.refreshInterval = refreshInterval
		if p.refreshInterval <= 0 {
			return
		}

		p.stopCh = make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		p.cancelFunc = cancel
		p.wg.Add(1)
		go p.refreshLoop(ctx, client)
	})
}

// StopRefreshing stops the periodic refresh goroutine.
// It is safe to call multiple times; only the first call stops the goroutine.
func (p *ResourceProvider) StopRefreshing() {
	p.stopOnce.Do(func() {
		if p.cancelFunc != nil {
			p.cancelFunc()
		}
		if p.stopCh != nil {
			close(p.stopCh)
			p.wg.Wait()
		}
		p.telemetry.Shutdown()
	})
}

func (p *ResourceProvider) refreshLoop(ctx context.Context, client *http.Client) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := p.Refresh(ctx, client)
			if err != nil {
				p.logger.Warn("resource refresh failed", zap.Error(err))
			}
		case <-p.stopCh:
			return
		}
	}
}
