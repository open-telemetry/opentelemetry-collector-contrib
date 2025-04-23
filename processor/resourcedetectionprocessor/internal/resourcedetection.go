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
	"time"

	backoff "github.com/cenkalti/backoff/v5"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

var allowErrorPropagationFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"processor.resourcedetection.propagateerrors",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, allows errors returned from resource detectors to propagate in the Start() method and stop the collector."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/37961"),
	featuregate.WithRegisterFromVersion("v0.121.0"),
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
	attributes []string,
	detectorConfigs ResourceDetectorConfig,
	detectorTypes ...DetectorType,
) (*ResourceProvider, error) {
	detectors, err := f.getDetectors(params, detectorConfigs, detectorTypes)
	if err != nil {
		return nil, err
	}

	attributesToKeep := make(map[string]struct{})
	if len(attributes) > 0 {
		for _, attribute := range attributes {
			attributesToKeep[attribute] = struct{}{}
		}
	}

	provider := NewResourceProvider(params.Logger, timeout, attributesToKeep, detectors...)
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
	detectedResource *resourceResult
	once             sync.Once
	attributesToKeep map[string]struct{}
}

type resourceResult struct {
	resource  pcommon.Resource
	schemaURL string
	err       error
}

func NewResourceProvider(logger *zap.Logger, timeout time.Duration, attributesToKeep map[string]struct{}, detectors ...Detector) *ResourceProvider {
	return &ResourceProvider{
		logger:           logger,
		timeout:          timeout,
		detectors:        detectors,
		attributesToKeep: attributesToKeep,
	}
}

func (p *ResourceProvider) Get(ctx context.Context, client *http.Client) (resource pcommon.Resource, schemaURL string, err error) {
	p.once.Do(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, client.Timeout)
		defer cancel()
		p.detectResource(ctx, client.Timeout)
	})

	return p.detectedResource.resource, p.detectedResource.schemaURL, p.detectedResource.err
}

func (p *ResourceProvider) detectResource(ctx context.Context, timeout time.Duration) {
	p.detectedResource = &resourceResult{}

	res := pcommon.NewResource()
	mergedSchemaURL := ""

	p.logger.Info("began detecting resource information")

	resultsChan := make([]chan resourceResult, len(p.detectors))
	for i, detector := range p.detectors {
		resultsChan[i] = make(chan resourceResult)
		go func(detector Detector) {
			sleep := backoff.ExponentialBackOff{
				InitialInterval:     1 * time.Second,
				RandomizationFactor: 1.5,
				Multiplier:          2,
				MaxInterval:         timeout,
			}
			sleep.Reset()
			var err error
			var r pcommon.Resource
			var schemaURL string
			for {
				r, schemaURL, err = detector.Detect(ctx)
				if err == nil {
					resultsChan[i] <- resourceResult{resource: r, schemaURL: schemaURL, err: nil}
					return
				}
				p.logger.Warn("failed to detect resource", zap.Error(err))

				timer := time.NewTimer(sleep.NextBackOff())
				select {
				case <-timer.C:
					fmt.Println("Retrying fetching data...")
				case <-ctx.Done():
					p.logger.Warn("Context was cancelled: %w", zap.Error(ctx.Err()))
					resultsChan[i] <- resourceResult{resource: r, schemaURL: schemaURL, err: err}
					return
				}
			}
		}(detector)
	}

	for _, ch := range resultsChan {
		result := <-ch
		if result.err != nil {
			if allowErrorPropagationFeatureGate.IsEnabled() {
				p.detectedResource.err = errors.Join(p.detectedResource.err, result.err)
			}
		} else {
			mergedSchemaURL = MergeSchemaURL(mergedSchemaURL, result.schemaURL)
			MergeResource(res, result.resource, false)
		}
	}

	droppedAttributes := filterAttributes(res.Attributes(), p.attributesToKeep)

	p.logger.Info("detected resource information", zap.Any("resource", res.Attributes().AsRaw()))
	if len(droppedAttributes) > 0 {
		p.logger.Info("dropped resource information", zap.Strings("resource keys", droppedAttributes))
	}

	p.detectedResource.resource = res
	p.detectedResource.schemaURL = mergedSchemaURL
}

func MergeSchemaURL(currentSchemaURL string, newSchemaURL string) string {
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

func filterAttributes(am pcommon.Map, attributesToKeep map[string]struct{}) []string {
	if len(attributesToKeep) > 0 {
		var droppedAttributes []string
		am.RemoveIf(func(k string, _ pcommon.Value) bool {
			_, keep := attributesToKeep[k]
			if !keep {
				droppedAttributes = append(droppedAttributes, k)
			}
			return !keep
		})
		return droppedAttributes
	}
	return nil
}

func MergeResource(to, from pcommon.Resource, overrideTo bool) {
	if IsEmptyResource(from) {
		return
	}

	toAttr := to.Attributes()
	for k, v := range from.Attributes().All() {
		if overrideTo {
			v.CopyTo(toAttr.PutEmpty(k))
		} else {
			if _, found := toAttr.Get(k); !found {
				v.CopyTo(toAttr.PutEmpty(k))
			}
		}
	}
}

func IsEmptyResource(res pcommon.Resource) bool {
	return res.Attributes().Len() == 0
}
