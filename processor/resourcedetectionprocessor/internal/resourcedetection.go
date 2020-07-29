// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package internal contains an interface for detecting resource information,
// and a provider to merge the resources returned by a slice of custom detectors.
package internal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type DetectorType string

type Detector interface {
	Detect(ctx context.Context) (pdata.Resource, error)
}

type ResourceProviderFactory struct {
	// detectors holds all possible detector types.
	detectors map[DetectorType]Detector
}

func NewProviderFactory(detectors map[DetectorType]Detector) *ResourceProviderFactory {
	return &ResourceProviderFactory{detectors: detectors}
}

func (f *ResourceProviderFactory) CreateResourceProvider(logger *zap.Logger, timeout time.Duration, detectorTypes ...DetectorType) (*ResourceProvider, error) {
	detectors, err := f.getDetectors(detectorTypes)
	if err != nil {
		return nil, err
	}

	provider := NewResourceProvider(logger, timeout, detectors...)
	return provider, nil
}

func (f *ResourceProviderFactory) getDetectors(detectorTypes []DetectorType) ([]Detector, error) {
	detectors := make([]Detector, 0, len(detectorTypes))
	for _, detectorType := range detectorTypes {
		detector, ok := f.detectors[detectorType]
		if !ok {
			return nil, fmt.Errorf("invalid detector key: %v", detectorType)
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
}

type resourceResult struct {
	resource pdata.Resource
	err      error
}

func NewResourceProvider(logger *zap.Logger, timeout time.Duration, detectors ...Detector) *ResourceProvider {
	return &ResourceProvider{
		logger:    logger,
		timeout:   timeout,
		detectors: detectors,
	}
}

func (p *ResourceProvider) Get(ctx context.Context) (pdata.Resource, error) {
	p.once.Do(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
		p.detectResource(ctx)
	})

	return p.detectedResource.resource, p.detectedResource.err
}

func (p *ResourceProvider) detectResource(ctx context.Context) {
	p.detectedResource = &resourceResult{}

	res := pdata.NewResource()
	res.InitEmpty()

	p.logger.Info("began detecting resource information")

	for _, detector := range p.detectors {
		r, err := detector.Detect(ctx)
		if err != nil {
			p.detectedResource.err = err
			return
		}

		MergeResource(res, r, false)
	}

	p.logger.Info("detected resource information", zap.Any("resource", resourceToMap(res)))

	p.detectedResource.resource = res
}

func resourceToMap(resource pdata.Resource) map[string]interface{} {
	mp := make(map[string]interface{}, resource.Attributes().Len())

	resource.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		switch v.Type() {
		case pdata.AttributeValueBOOL:
			mp[k] = v.BoolVal()
		case pdata.AttributeValueINT:
			mp[k] = v.IntVal()
		case pdata.AttributeValueDOUBLE:
			mp[k] = v.DoubleVal()
		case pdata.AttributeValueSTRING:
			mp[k] = v.StringVal()
		}
	})

	return mp
}

func MergeResource(to, from pdata.Resource, overrideTo bool) {
	if IsEmptyResource(from) {
		return
	}

	toAttr := to.Attributes()
	from.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		if overrideTo {
			toAttr.Upsert(k, v)
		} else {
			toAttr.Insert(k, v)
		}
	})
}

func IsEmptyResource(res pdata.Resource) bool {
	return res.IsNil() || res.Attributes().Len() == 0
}
