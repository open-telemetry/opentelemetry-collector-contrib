// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.uber.org/zap"
)

// Entity types emitted by detectors. Semantic conventions for entities are
// still in development; these names follow the current entities draft.
const (
	EntityTypeHost         = "host"
	EntityTypeK8sCluster   = "k8s.cluster"
	EntityTypeK8sNode      = "k8s.node"
	EntityTypeCloudAccount = "cloud.account"
)

// DetectedEntity describes an entity contributed by a detector. The
// identifying and descriptive attributes are referenced by key and must be
// present in the resource returned by the same detector.
type DetectedEntity struct {
	SchemaURL       string
	Type            string
	IDKeys          []string
	DescriptionKeys []string

	// IDContextTypeCandidates lists, in priority order, the entity types that
	// can establish the uniqueness context of this entity's ID. It must be
	// empty for entities with globally unique IDs.
	IDContextTypeCandidates []string

	idContextType string
}

// EntityDetector is implemented by detectors that can declare which entities
// their detected resource attributes describe.
type EntityDetector interface {
	Detector
	EntityRefs(resource pcommon.Resource) []DetectedEntity
}

func resolveEntities(entities []DetectedEntity, logger *zap.Logger) []DetectedEntity {
	merged := make([]DetectedEntity, 0, len(entities))
	byType := make(map[string]int, len(entities))
	for _, ent := range entities {
		if ent.Type == "" || len(ent.IDKeys) == 0 {
			logger.Warn("skipping entity without type or id keys", zap.String("type", ent.Type))
			continue
		}
		i, ok := byType[ent.Type]
		if !ok {
			byType[ent.Type] = len(merged)
			merged = append(merged, ent)
			continue
		}
		// Same entity type from a lower-precedence detector: the first
		// declaration keeps the identity, description keys are unioned.
		merged[i].DescriptionKeys = unionKeys(merged[i].DescriptionKeys, ent.DescriptionKeys)
	}

	for i := range merged {
		for _, candidate := range merged[i].IDContextTypeCandidates {
			if _, ok := byType[candidate]; ok && candidate != merged[i].Type {
				merged[i].idContextType = candidate
				break
			}
		}
	}

	breakContextCycles(merged, byType, logger)
	return merged
}

func breakContextCycles(entities []DetectedEntity, byType map[string]int, logger *zap.Logger) {
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, t := range types {
		visited := map[string]bool{}
		i := byType[t]
		for entities[i].idContextType != "" {
			if visited[entities[i].Type] {
				logger.Warn("entity ID context chain forms a cycle, dropping context",
					zap.String("entity.type", entities[i].Type),
					zap.String("entity.id_context_type", entities[i].idContextType))
				entities[i].idContextType = ""
				break
			}
			visited[entities[i].Type] = true
			next, ok := byType[entities[i].idContextType]
			if !ok {
				break
			}
			i = next
		}
	}
}

func appendEntityRefs(res pcommon.Resource, entities []DetectedEntity, logger *zap.Logger) {
	attrs := res.Attributes()
	entityMap := xentity.ResourceEntities(res)
	for _, ent := range entities {
		if !hasAllKeys(attrs, ent.IDKeys) {
			logger.Warn("skipping entity: some id keys are missing in the detected resource",
				zap.String("entity.type", ent.Type), zap.Strings("entity.id_keys", ent.IDKeys))
			continue
		}
		entity := entityMap.PutEmpty(ent.Type)
		entity.SetSchemaURL(ent.SchemaURL)
		entity.SetIDContextType(ent.idContextType)
		for _, k := range ent.IDKeys {
			copyResourceAttribute(attrs, entity.IdentifyingAttributes(), k)
		}
		for _, k := range ent.DescriptionKeys {
			copyResourceAttribute(attrs, entity.DescriptiveAttributes(), k)
		}
	}
}

func copyResourceAttribute(attrs pcommon.Map, target xentity.EntityAttributeMap, key string) bool {
	val, ok := attrs.Get(key)
	if !ok {
		return false
	}
	copied := pcommon.NewValueEmpty()
	val.CopyTo(copied)
	copied.CopyTo(target.PutEmpty(key))
	return true
}

// MergeEntityRefs copies entity refs from one resource to another after the
// resource attributes have already been merged.
func MergeEntityRefs(to, from pcommon.Resource, overrideTo bool) {
	fromRefs := xentity.ResourceEntityRefs(from)
	if fromRefs.Len() == 0 {
		return
	}
	toRefs := xentity.ResourceEntityRefs(to)
	if overrideTo {
		fromTypes := make(map[string]bool, fromRefs.Len())
		for i := 0; i < fromRefs.Len(); i++ {
			ref := fromRefs.At(i)
			if hasAllKeys(to.Attributes(), ref.IdKeys().AsRaw()) {
				fromTypes[ref.Type()] = true
			}
		}
		toRefs.RemoveIf(func(ref xentity.EntityRef) bool {
			return fromTypes[ref.Type()]
		})
	}

	existing := make(map[string]bool, toRefs.Len())
	for i := 0; i < toRefs.Len(); i++ {
		existing[toRefs.At(i).Type()] = true
	}
	for i := 0; i < fromRefs.Len(); i++ {
		ref := fromRefs.At(i)
		if existing[ref.Type()] || !hasAllKeys(to.Attributes(), ref.IdKeys().AsRaw()) {
			continue
		}
		ref.CopyTo(toRefs.AppendEmpty())
		existing[ref.Type()] = true
	}
}

func hasAllKeys(attrs pcommon.Map, keys []string) bool {
	for _, k := range keys {
		if _, ok := attrs.Get(k); !ok {
			return false
		}
	}
	return true
}

func unionKeys(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, k := range a {
		seen[k] = true
	}
	for _, k := range b {
		if !seen[k] {
			seen[k] = true
			a = append(a, k)
		}
	}
	return a
}
