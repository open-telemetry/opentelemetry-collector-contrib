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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
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

type DetectorFactory func(processor.Settings, DetectorConfig) (Detector, error)

type ResourceProviderFactory struct {
	// detectors holds all possible detector types.
	detectors map[DetectorType]DetectorFactory
}

func NewProviderFactory(detectors map[DetectorType]DetectorFactory) *ResourceProviderFactory {
	return &ResourceProviderFactory{detectors: detectors}
}

func (f *ResourceProviderFactory) CreateResourceProvider(
	params processor.Settings,
	timeout time.Duration,
	detectorConfigs ResourceDetectorConfig,
	detectorTypes ...DetectorType,
) (*ResourceProvider, error) {
	detectors, err := f.getDetectors(params, detectorConfigs, detectorTypes)
	if err != nil {
		return nil, err
	}

	provider := NewResourceProvider(params.Logger, timeout, detectors...)
	return provider, nil
}

func (f *ResourceProviderFactory) getDetectors(params processor.Settings, detectorConfigs ResourceDetectorConfig, detectorTypes []DetectorType) ([]Detector, error) {
	detectors := make([]Detector, 0, len(detectorTypes))
	for _, detectorType := range detectorTypes {
		detectorFactory, ok := f.detectors[detectorType]
		if !ok {
			return nil, fmt.Errorf("invalid detector key: %v", detectorType)
		}

		detector, err := detectorFactory(params, detectorConfigs.GetConfigFromType(detectorType))
		if err != nil {
			return nil, fmt.Errorf("failed creating detector type %q: %w", detectorType, err)
		}

		detectors = append(detectors, detector)
	}

	return detectors, nil
}

type ResourceProvider struct {
	logger           *zap.Logger
	timeout          time.Duration
	detectors        []Detector
	detectedResource atomic.Pointer[resourceResult]

	// Refresh loop control
	refreshInterval time.Duration
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

type resourceResult struct {
	resource  pcommon.Resource
	schemaURL string
	err       error
}

func NewResourceProvider(logger *zap.Logger, timeout time.Duration, detectors ...Detector) *ResourceProvider {
	return &ResourceProvider{
		logger:          logger,
		timeout:         timeout,
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
	ctx, cancel := context.WithTimeout(ctx, client.Timeout)
	defer cancel()

	res, schemaURL, err := p.detectResource(ctx)
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

func (p *ResourceProvider) detectResource(ctx context.Context) (pcommon.Resource, string, error) {
	res := pcommon.NewResource()
	mergedSchemaURL := ""
	var joinedErr error
	successes := 0

	p.logger.Info("began detecting resource information")

	resultsChan := make([]chan resourceResult, len(p.detectors))
	for i, detector := range p.detectors {
		ch := make(chan resourceResult, 1)
		resultsChan[i] = ch

		go func(detector Detector, ch chan resourceResult) {
			sleep := backoff.ExponentialBackOff{
				InitialInterval:     1 * time.Second,
				RandomizationFactor: 1.5,
				Multiplier:          2,
			}
			sleep.Reset()

			for {
				r, schemaURL, err := detector.Detect(ctx)
				if err == nil {
					ch <- resourceResult{resource: r, schemaURL: schemaURL}
					return
				}

				p.logger.Warn("failed to detect resource", zap.Error(err))

				next := sleep.NextBackOff()
				if next == backoff.Stop {
					ch <- resourceResult{err: err}
					return
				}

				timer := time.NewTimer(next)
				select {
				case <-ctx.Done():
					p.logger.Warn("context was cancelled", zap.Error(ctx.Err()))
					timer.Stop()
					ch <- resourceResult{err: err}
					return
				case <-timer.C:
					// retry
				}
			}
		}(detector, ch)
	}

	for _, ch := range resultsChan {
		result := <-ch
		if result.err != nil {
			if metadata.ProcessorResourcedetectionPropagateerrorsFeatureGate.IsEnabled() {
				joinedErr = errors.Join(joinedErr, result.err)
			}
			continue
		}
		successes++
		mergedSchemaURL = MergeSchemaURL(mergedSchemaURL, result.schemaURL)
		MergeResource(res, result.resource, false)
	}

	p.logger.Info("detected resource information", zap.Any("resource", res.Attributes().AsRaw()))

	// Determine the error to return based on feature gate setting.
	var returnErr error
	if metadata.ProcessorResourcedetectionPropagateerrorsFeatureGate.IsEnabled() {
		// Feature gate enabled: return joined errors (if any)
		if successes == 0 && joinedErr == nil {
			returnErr = errors.New("resource detection failed: no detectors succeeded")
		} else {
			returnErr = joinedErr
		}
	}

	// If all detectors failed, return empty resource.
	if successes == 0 {
		if !metadata.ProcessorResourcedetectionPropagateerrorsFeatureGate.IsEnabled() {
			p.logger.Warn("resource detection failed but error propagation is disabled")
		}
		return pcommon.NewResource(), "", returnErr
	}

	// Partial or full success: return merged resources.
	return res, mergedSchemaURL, returnErr
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

// StartRefreshing begins periodic resource refresh if refreshInterval > 0
func (p *ResourceProvider) StartRefreshing(refreshInterval time.Duration, client *http.Client) {
	p.refreshInterval = refreshInterval
	if p.refreshInterval <= 0 {
		return
	}

	p.stopCh = make(chan struct{})
	p.wg.Add(1)
	go p.refreshLoop(client)
}

// StopRefreshing stops the periodic refresh goroutine
func (p *ResourceProvider) StopRefreshing() {
	if p.stopCh != nil {
		close(p.stopCh)
		p.wg.Wait()
	}
}

func (p *ResourceProvider) refreshLoop(client *http.Client) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := p.Refresh(context.Background(), client)
			if err != nil {
				p.logger.Warn("resource refresh failed", zap.Error(err))
			}
		case <-p.stopCh:
			return
		}
	}
}
