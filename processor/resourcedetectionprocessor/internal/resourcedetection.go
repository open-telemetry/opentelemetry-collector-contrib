// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package internal contains an interface for detecting resource information,
// and a provider to merge the resources returned by a slice of custom detectors.
package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pentity"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
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
	detectorTypes ...DetectorType) (*ResourceProvider, error) {
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
		p.detectResource(ctx)
	})

	return p.detectedResource.resource, p.detectedResource.schemaURL, p.detectedResource.err
}

func (p *ResourceProvider) detectResource(ctx context.Context) {
	p.detectedResource = &resourceResult{}

	res := pcommon.NewResource()
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

	droppedAttributes := filterAttributes(res.Attributes(), p.attributesToKeep)

	p.logger.Info("detected resource information", zap.Any("resource", res.Attributes().AsRaw()))
	if len(droppedAttributes) > 0 {
		p.logger.Info("dropped resource information", zap.Strings("resource keys", droppedAttributes))
	}
	if res.Entities().Len() > 0 {
		types := make([]string, 0, res.Entities().Len())
		for i := 0; i < res.Entities().Len(); i++ {
			types = append(types, res.Entities().At(i).Type())
		}
		p.logger.Info("detected entities: " + strings.Join(types, ", "))
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
	from.Attributes().Range(func(k string, v pcommon.Value) bool {
		if overrideTo {
			v.CopyTo(toAttr.PutEmpty(k))
		} else {
			if _, found := toAttr.Get(k); !found {
				v.CopyTo(toAttr.PutEmpty(k))
			}
		}
		return true
	})

	for i := 0; i < from.Entities().Len(); i++ {
		fromEntity := from.Entities().At(i)
		isNewEntity := true
		for j := 0; j < to.Entities().Len(); j++ {
			toEntity := to.Entities().At(j)
			if toEntity.Type() == fromEntity.Type() {
				isNewEntity = false
				MergeEntity(toEntity, fromEntity, overrideTo)
			}
		}
		if isNewEntity {
			fromEntity.CopyTo(to.Entities().AppendEmpty())
		}
	}
}

func IsEmptyResource(res pcommon.Resource) bool {
	return res.Attributes().Len() == 0
}

func MergeEntity(to, from pcommon.ResourceEntityRef, overrideTo bool) {
	fromID := from.IdAttrKeys().AsRaw()
	toID := to.IdAttrKeys().AsRaw()
	idSuperset := slicesUnion(fromID, toID)
	// We assume that the entity IDs are the same if one of them is a superset of the other.
	if len(idSuperset) == len(fromID) || len(idSuperset) == len(toID) {
		to.IdAttrKeys().FromRaw(idSuperset)
		to.DescrAttrKeys().FromRaw(slicesUnion(from.DescrAttrKeys().AsRaw(), to.DescrAttrKeys().AsRaw()))
	} else if overrideTo {
		// If the entity IDs are different, we are not able to merge them.
		// We pick one of them and ignore the other based on the overrideTo flag.
		from.CopyTo(to)
	}
}

func slicesUnion(a, b []string) []string {
	slices.Sort(a)
	slices.Sort(b)
	i, j := 0, 0
	var result []string

	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			result = append(result, a[i])
			i++
		} else if a[i] > b[j] {
			result = append(result, b[j])
			j++
		} else { // a[i] == b[j]
			result = append(result, a[i])
			i++
			j++
		}
	}

	// Append remaining elements from either slice
	for i < len(a) {
		result = append(result, a[i])
		i++
	}
	for j < len(b) {
		result = append(result, b[j])
		j++
	}

	return result
}

// MergeEntityRef merges information from one entity reference to entity event if the entity IDs match (one is a superset of the other).
func MergeEntityRef(to pentity.EntityEvent, from pcommon.ResourceEntityRef, fromAttrs pcommon.Map, overrideTo bool) {
	fromAttrsRaw := fromAttrs.AsRaw()
	fromID := map[string]any{}
	for _, key := range from.IdAttrKeys().AsRaw() {
		val, ok := fromAttrsRaw[key]
		if ok {
			fromID[key] = val
		}
	}
	toID := to.Id().AsRaw()
	idSuperset, match := mapUnion(fromID, toID)
	// We assume that the entity IDs are the same if one of them is a superset of the other.
	if !match {
		return
	}
	if len(idSuperset) != len(toID) {
		_ = to.Id().FromRaw(idSuperset)
	}
	for i := 0; i < from.DescrAttrKeys().Len(); i++ {
		key := from.DescrAttrKeys().At(i)
		valFrom, okFrom := fromAttrs.Get(key)
		_, okTo := to.EntityState().Attributes().Get(key)
		if okFrom && (overrideTo || !okTo) {
			valFrom.CopyTo(to.EntityState().Attributes().PutEmpty(key))
		}
	}
}

// mapUnion returns the union of two maps. If the maps have the same key with different values, it returns false.
func mapUnion(a, b map[string]any) (map[string]any, bool) {
	result := map[string]any{}
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		val, ok := result[k]
		if ok && val != v {
			return nil, false
		}
		result[k] = v
	}
	return result, true
}

func RemoveInvalidEntities(re pentity.ResourceEntities) {
	for i := 0; i < re.ScopeEntities().Len(); i++ {
		re.Resource().Entities().RemoveIf(func(entity pcommon.ResourceEntityRef) bool {
			return entity.Type() == "" || entity.IdAttrKeys().Len() == 0
		})
	}
}
