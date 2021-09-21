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
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type DetectorType string

type Detector interface {
	Detect(ctx context.Context) (resource pdata.Resource, schemaURL string, err error)
}

type DetectorConfig interface{}

type ResourceDetectorConfig interface {
	GetConfigFromType(DetectorType) DetectorConfig
}

type DetectorFactory func(component.ProcessorCreateSettings, DetectorConfig) (Detector, error)

type ResourceProviderFactory struct {
	// detectors holds all possible detector types.
	detectors map[DetectorType]DetectorFactory
}

func NewProviderFactory(detectors map[DetectorType]DetectorFactory) *ResourceProviderFactory {
	return &ResourceProviderFactory{detectors: detectors}
}

func (f *ResourceProviderFactory) CreateResourceProvider(
	params component.ProcessorCreateSettings,
	timeout time.Duration,
	detectorConfigs ResourceDetectorConfig,
	detectorTypes ...DetectorType) (*ResourceProvider, error) {
	detectors, err := f.getDetectors(params, detectorConfigs, detectorTypes)
	if err != nil {
		return nil, err
	}

	provider := NewResourceProvider(params.Logger, timeout, detectors...)
	return provider, nil
}

func (f *ResourceProviderFactory) getDetectors(params component.ProcessorCreateSettings, detectorConfigs ResourceDetectorConfig, detectorTypes []DetectorType) ([]Detector, error) {
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
}

type resourceResult struct {
	resource  pdata.Resource
	schemaURL string
	err       error
}

func NewResourceProvider(logger *zap.Logger, timeout time.Duration, detectors ...Detector) *ResourceProvider {
	return &ResourceProvider{
		logger:    logger,
		timeout:   timeout,
		detectors: detectors,
	}
}

func (p *ResourceProvider) Get(ctx context.Context) (resource pdata.Resource, schemaURL string, err error) {
	p.once.Do(func() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
		p.detectResource(ctx)
	})

	return p.detectedResource.resource, p.detectedResource.schemaURL, p.detectedResource.err
}

func (p *ResourceProvider) detectResource(ctx context.Context) {
	p.detectedResource = &resourceResult{}

	res := pdata.NewResource()
	mergedSchemaURL := ""

	p.logger.Info("began detecting resource information")

	for _, detector := range p.detectors {
		r, schemaURL, err := detector.Detect(ctx)
		if err != nil {
			p.logger.Warn("failed to detect resource", zap.Error(err))
		} else {
			mergedSchemaURL = MergeSchemaURL(mergedSchemaURL, schemaURL)
			MergeResource(res, r, false)
		}

	}

	p.logger.Info("detected resource information", zap.Any("resource", AttributesToMap(res.Attributes())))

	p.detectedResource.resource = res
	p.detectedResource.schemaURL = mergedSchemaURL
}

func AttributesToMap(am pdata.AttributeMap) map[string]interface{} {
	mp := make(map[string]interface{}, am.Len())
	am.Range(func(k string, v pdata.AttributeValue) bool {
		mp[k] = UnwrapAttribute(v)
		return true
	})
	return mp
}

func UnwrapAttribute(v pdata.AttributeValue) interface{} {
	switch v.Type() {
	case pdata.AttributeValueTypeBool:
		return v.BoolVal()
	case pdata.AttributeValueTypeInt:
		return v.IntVal()
	case pdata.AttributeValueTypeDouble:
		return v.DoubleVal()
	case pdata.AttributeValueTypeString:
		return v.StringVal()
	case pdata.AttributeValueTypeArray:
		return getSerializableArray(v.ArrayVal())
	case pdata.AttributeValueTypeMap:
		return AttributesToMap(v.MapVal())
	default:
		return nil
	}
}

func getSerializableArray(inArr pdata.AnyValueArray) []interface{} {
	var outArr []interface{}
	for i := 0; i < inArr.Len(); i++ {
		outArr = append(outArr, UnwrapAttribute(inArr.At(i)))
	}

	return outArr
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

func MergeResource(to, from pdata.Resource, overrideTo bool) {
	if IsEmptyResource(from) {
		return
	}

	toAttr := to.Attributes()
	from.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		if overrideTo {
			toAttr.Upsert(k, v)
		} else {
			toAttr.Insert(k, v)
		}
		return true
	})
}

func IsEmptyResource(res pdata.Resource) bool {
	return res.Attributes().Len() == 0
}

// GOOSToOSType maps a runtime.GOOS-like value to os.type style.
func GOOSToOSType(goos string) string {
	switch goos {
	case "dragonfly":
		return "DRAGONFLYBSD"
	}
	return strings.ToUpper(goos)
}
