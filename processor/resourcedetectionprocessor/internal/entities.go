// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.uber.org/zap"
)

// Entity types emitted by detectors. Entity semantic conventions are still in
// development; these follow the current entities draft.
const (
	EntityTypeCloud                 = "cloud"
	EntityTypeCloudRegion           = "cloud.region"
	EntityTypeCloudAvailabilityZone = "cloud.availability_zone"
	EntityTypeFaas                  = "faas"
	EntityTypeHost                  = "host"
	EntityTypeK8sCluster            = "k8s.cluster"
	EntityTypeK8sNode               = "k8s.node"
)

// CloudEntities declares the cloud location hierarchy present on a detected
// resource, using only attributes that already exist in semantic conventions:
//
//	cloud                   ID: cloud.provider, cloud.account.id   global
//	cloud.region            ID: cloud.region                        local within cloud
//	cloud.availability_zone ID: cloud.availability_zone             local within cloud
//
// cloud.region and cloud.availability_zone become identities of their own
// entities instead of descriptive attributes of cloud. That is the whole
// change needed for cloud-scoped IDs: an EC2 instance ID is unique within a
// region, a GKE cluster name within a project and a zone or region. Each of
// those is now a single scope entity type whose own chain carries the account.
// Zones are scoped by cloud rather than by cloud.region because AWS zone
// names are per-account mappings and GCP zone names are unique on their own;
// either way the account is enough, and a zone can be declared without a
// region.
func CloudEntities(attrs pcommon.Map, schemaURL string) []DetectedEntity {
	if !HasKeys(attrs, string(conventions.CloudProviderKey), string(conventions.CloudAccountIDKey)) {
		return nil
	}
	entities := []DetectedEntity{{
		SchemaURL:       schemaURL,
		Type:            EntityTypeCloud,
		IDKeys:          []string{string(conventions.CloudProviderKey), string(conventions.CloudAccountIDKey)},
		IDScope:         xentity.IdScopeGlobal,
		DescriptionKeys: PresentKeys(attrs, string(conventions.CloudPlatformKey)),
	}}
	if HasKeys(attrs, string(conventions.CloudRegionKey)) {
		entities = append(entities, DetectedEntity{
			SchemaURL:         schemaURL,
			Type:              EntityTypeCloudRegion,
			IDKeys:            []string{string(conventions.CloudRegionKey)},
			IDScope:           xentity.IdScopeLocal,
			IDScopeEntityType: EntityTypeCloud,
		})
	}
	if HasKeys(attrs, string(conventions.CloudAvailabilityZoneKey)) {
		entities = append(entities, DetectedEntity{
			SchemaURL:         schemaURL,
			Type:              EntityTypeCloudAvailabilityZone,
			IDKeys:            []string{string(conventions.CloudAvailabilityZoneKey)},
			IDScope:           xentity.IdScopeLocal,
			IDScopeEntityType: EntityTypeCloud,
		})
	}
	return entities
}

// ScopedBy returns a local scope by the first of the candidate entity types
// whose identifying attribute is present, or unspecified when none is: the
// detector then knows the ID is scoped but cannot name the scope.
func ScopedBy(attrs pcommon.Map, candidates ...struct{ Type, Key string }) (xentity.IdScope, string) {
	for _, c := range candidates {
		if HasKeys(attrs, c.Key) {
			return xentity.IdScopeLocal, c.Type
		}
	}
	return xentity.IdScopeUnspecified, ""
}

// Candidate scope entity types with the attribute that identifies them.
var (
	ScopeCloud                 = struct{ Type, Key string }{EntityTypeCloud, string(conventions.CloudAccountIDKey)}
	ScopeCloudRegion           = struct{ Type, Key string }{EntityTypeCloudRegion, string(conventions.CloudRegionKey)}
	ScopeCloudAvailabilityZone = struct{ Type, Key string }{EntityTypeCloudAvailabilityZone, string(conventions.CloudAvailabilityZoneKey)}
)

// DetectedEntity describes an entity contributed by a detector. Identifying and
// descriptive attributes are referenced by key and must be present in the
// resource returned by the same detector.
//
// IDScope and IDScopeEntityType describe the entity's own ID. The detector that
// produced the entity is the only component allowed to set them. They say what
// kind of entity the ID is unique within, not which particular entity, so they
// do not depend on which other detectors are configured or on their order.
type DetectedEntity struct {
	SchemaURL       string
	Type            string
	IDKeys          []string
	DescriptionKeys []string

	// IDScope says whether the ID is unique on its own (global), only within an
	// entity of type IDScopeEntityType (local), or whether the detector cannot
	// tell (unspecified).
	IDScope xentity.IdScope
	// IDScopeEntityType is the type of the entity the ID is unique within. Must
	// be set when IDScope is local and empty otherwise.
	IDScopeEntityType string
}

// EntityDetector is implemented by detectors that can declare which entities
// their detected resource attributes describe.
type EntityDetector interface {
	Detector
	// Entities returns the entities described by the resource that Detect
	// returned. The resource passed in is that same resource.
	Entities(resource pcommon.Resource) []DetectedEntity
}

// attachDetectedEntities adds the entities a detector declared to the merged
// resource. Attributes were already merged with first-detector precedence, so
// an entity is attached only if every identifying attribute on the merged
// resource still carries the value this detector produced. Otherwise a
// higher-precedence detector changed the identity and this declaration no
// longer describes what is on the resource.
//
// Entities of the same type from different detectors are merged when the
// entities data model allows it (same identity, schema URL, ID scope and scope
// entity type), otherwise the earlier declaration wins, consistent with
// attribute precedence. The losing declaration's attributes stay on the
// resource as unassociated attributes.
func attachDetectedEntities(merged, detected pcommon.Resource, entities []DetectedEntity, logger *zap.Logger) {
	mergedAttrs := merged.Attributes()
	detectedAttrs := detected.Attributes()
	refs := xentity.ResourceEntityRefs(merged)

	valid := make([]DetectedEntity, 0, len(entities))
	for _, ent := range entities {
		if validDeclaration(ent, logger) {
			valid = append(valid, ent)
		}
	}
	holds := identityHolds(valid, mergedAttrs, detectedAttrs)

	for _, ent := range valid {
		if !holds[ent.Type] {
			logger.Debug("skipping entity: a higher-precedence detector set a different value for an identifying attribute of this entity or of an entity scoping it",
				zap.String("entity.type", ent.Type), zap.Strings("entity.id_keys", ent.IDKeys))
			continue
		}
		owned := ownedKeys(refs)
		descKeys := make([]string, 0, len(ent.DescriptionKeys))
		for _, k := range ent.DescriptionKeys {
			if owner, ok := owned[k]; ok && owner != ent.Type {
				continue
			}
			if sameValues(mergedAttrs, detectedAttrs, []string{k}) {
				descKeys = append(descKeys, k)
			}
		}

		if existing, ok := findRef(refs, ent.Type); ok {
			if canMergeRef(existing, ent) {
				unionDescriptionKeys(existing, descKeys)
			} else {
				logger.Info("skipping entity: a higher-precedence detector declared the same entity type with a different identity",
					zap.String("entity.type", ent.Type),
					zap.Strings("kept.id_keys", existing.IdKeys().AsRaw()),
					zap.Strings("dropped.id_keys", ent.IDKeys))
			}
			continue
		}
		putRef(refs.AppendEmpty(), ent, descKeys)
	}
}

// MergeResourceAndEntities merges the detected resource `from` into the
// telemetry resource `to`, attributes first, then entity refs, following the
// Resource data model's rules for merging an entity into a Resource:
//
//   - a detected entity whose identifying attributes did not end up on `to`
//     (overrideTo is false and `to` already had different values) is skipped;
//   - a detected entity that can be merged with an existing entity of the same
//     type (same identity, schema URL, ID scope and scope entity type) merges
//     its description keys into it;
//   - otherwise an existing entity of the same type is replaced when
//     overrideTo is set and kept when it is not;
//   - when overrideTo overwrites an attribute referenced by an existing entity
//     of another type, that entity's ref is dropped and its remaining
//     attributes stay unassociated.
//
// The ID scope fields of entities already on `to` are never modified, and the
// detected entities' fields are copied verbatim: this processor is the producer
// of the detected entities only.
func MergeResourceAndEntities(to, from pcommon.Resource, overrideTo bool) {
	if IsEmptyResource(from) {
		return
	}
	toAttrs := to.Attributes()
	fromAttrs := from.Attributes()
	toRefs := xentity.ResourceEntityRefs(to)
	fromRefs := xentity.ResourceEntityRefs(from)

	ownedBefore := ownedKeys(toRefs)
	dropped := map[string]bool{}
	for k, v := range fromAttrs.All() {
		if overrideTo {
			if cur, found := toAttrs.Get(k); found && !cur.Equal(v) {
				if owner, ok := ownedBefore[k]; ok {
					dropped[owner] = true
				}
			}
			v.CopyTo(toAttrs.PutEmpty(k))
		} else if target, found := toAttrs.GetOrPutEmpty(k); !found {
			v.CopyTo(target)
		}
	}
	if len(dropped) > 0 {
		toRefs.RemoveIf(func(ref xentity.EntityRef) bool { return dropped[ref.Type()] })
	}

	detected := make([]DetectedEntity, 0, fromRefs.Len())
	for i := 0; i < fromRefs.Len(); i++ {
		src := fromRefs.At(i)
		detected = append(detected, DetectedEntity{
			SchemaURL:         src.SchemaUrl(),
			Type:              src.Type(),
			IDKeys:            src.IdKeys().AsRaw(),
			DescriptionKeys:   src.DescriptionKeys().AsRaw(),
			IDScope:           src.IdScope(),
			IDScopeEntityType: src.IdScopeEntityType(),
		})
	}
	holds := identityHolds(detected, toAttrs, fromAttrs)

	for _, ent := range detected {
		if !holds[ent.Type] {
			continue
		}
		owned := ownedKeys(toRefs)
		descKeys := make([]string, 0, len(ent.DescriptionKeys))
		for _, k := range ent.DescriptionKeys {
			if owner, ok := owned[k]; ok && owner != ent.Type {
				continue
			}
			if sameValues(toAttrs, fromAttrs, []string{k}) {
				descKeys = append(descKeys, k)
			}
		}
		if existing, ok := findRef(toRefs, ent.Type); ok {
			switch {
			case canMergeRef(existing, ent):
				unionDescriptionKeys(existing, descKeys)
				continue
			case overrideTo:
				toRefs.RemoveIf(func(ref xentity.EntityRef) bool { return ref.Type() == ent.Type })
			default:
				continue
			}
		}
		putRef(toRefs.AppendEmpty(), ent, descKeys)
	}
}

func validDeclaration(ent DetectedEntity, logger *zap.Logger) bool {
	switch {
	case ent.Type == "" || len(ent.IDKeys) == 0:
		logger.Warn("skipping entity without type or id keys", zap.String("entity.type", ent.Type))
		return false
	case ent.IDScope == xentity.IdScopeLocal && ent.IDScopeEntityType == "":
		logger.Warn("skipping entity: local id scope requires a scope entity type", zap.String("entity.type", ent.Type))
		return false
	case ent.IDScope != xentity.IdScopeLocal && ent.IDScopeEntityType != "":
		logger.Warn("skipping entity: scope entity type is only allowed with local id scope",
			zap.String("entity.type", ent.Type), zap.String("entity.id_scope", ent.IDScope.String()))
		return false
	case ent.IDScopeEntityType == ent.Type:
		logger.Warn("skipping entity: an entity cannot be scoped by its own type", zap.String("entity.type", ent.Type))
		return false
	}
	seen := map[string]bool{}
	for _, k := range ent.IDKeys {
		if seen[k] {
			logger.Warn("skipping entity: duplicate id key", zap.String("entity.type", ent.Type), zap.String("key", k))
			return false
		}
		seen[k] = true
	}
	for _, k := range ent.DescriptionKeys {
		if seen[k] {
			logger.Warn("skipping entity: key is both identifying and descriptive", zap.String("entity.type", ent.Type), zap.String("key", k))
			return false
		}
	}
	return true
}

// identityHolds reports, per entity type, whether the entity's identity is
// what its producer saw: every identifying attribute has the producer's value
// on the target resource, and so does every entity in its scope chain that
// the same producer declared. Without the chain condition a local entity
// could be attached next to a scoping entity of the same type that came from
// somewhere else, and a consumer would compose a Global Identity the producer
// never asserted. A scoping entity the producer did not declare is not
// checked: it may legitimately be absent or come from another producer.
func identityHolds(entities []DetectedEntity, target, source pcommon.Map) map[string]bool {
	byType := make(map[string]DetectedEntity, len(entities))
	self := make(map[string]bool, len(entities))
	for _, ent := range entities {
		byType[ent.Type] = ent
		self[ent.Type] = sameValues(target, source, ent.IDKeys)
	}
	holds := make(map[string]bool, len(entities))
	for _, ent := range entities {
		ok := true
		visited := map[string]bool{}
		for cur := ent; ; {
			if visited[cur.Type] {
				break
			}
			visited[cur.Type] = true
			if !self[cur.Type] {
				ok = false
				break
			}
			if cur.IDScope != xentity.IdScopeLocal {
				break
			}
			next, declared := byType[cur.IDScopeEntityType]
			if !declared {
				break
			}
			cur = next
		}
		holds[ent.Type] = ok
	}
	return holds
}

// sameValues reports whether every key is present in both maps with equal values.
func sameValues(a, b pcommon.Map, keys []string) bool {
	for _, k := range keys {
		av, ok := a.Get(k)
		if !ok {
			return false
		}
		bv, ok := b.Get(k)
		if !ok || !av.Equal(bv) {
			return false
		}
	}
	return true
}

// ownedKeys maps every attribute key referenced by an entity ref to the type of that entity.
func ownedKeys(refs xentity.EntityRefSlice) map[string]string {
	owned := map[string]string{}
	for i := 0; i < refs.Len(); i++ {
		ref := refs.At(i)
		for _, k := range ref.IdKeys().All() {
			owned[k] = ref.Type()
		}
		for _, k := range ref.DescriptionKeys().All() {
			owned[k] = ref.Type()
		}
	}
	return owned
}

func findRef(refs xentity.EntityRefSlice, entityType string) (xentity.EntityRef, bool) {
	for i := 0; i < refs.Len(); i++ {
		if refs.At(i).Type() == entityType {
			return refs.At(i), true
		}
	}
	return xentity.EntityRef{}, false
}

// canMergeRef implements the entities data model's can_merge check. Identifying
// attribute values are not compared because both sides reference the same
// resource attributes: equal key sets imply equal values.
func canMergeRef(existing xentity.EntityRef, ent DetectedEntity) bool {
	if existing.SchemaUrl() != ent.SchemaURL ||
		existing.IdScope() != ent.IDScope ||
		existing.IdScopeEntityType() != ent.IDScopeEntityType {
		return false
	}
	have := existing.IdKeys().AsRaw()
	if len(have) != len(ent.IDKeys) {
		return false
	}
	for _, k := range ent.IDKeys {
		if !slices.Contains(have, k) {
			return false
		}
	}
	return true
}

func unionDescriptionKeys(ref xentity.EntityRef, keys []string) {
	have := ref.DescriptionKeys()
	ids := ref.IdKeys()
	for _, k := range keys {
		if slices.Contains(have.AsRaw(), k) || slices.Contains(ids.AsRaw(), k) {
			continue
		}
		have.Append(k)
	}
}

func putRef(ref xentity.EntityRef, ent DetectedEntity, descKeys []string) {
	ref.SetType(ent.Type)
	ref.SetSchemaUrl(ent.SchemaURL)
	ref.SetIdScope(ent.IDScope)
	ref.SetIdScopeEntityType(ent.IDScopeEntityType)
	ref.IdKeys().FromRaw(ent.IDKeys)
	ref.DescriptionKeys().FromRaw(descKeys)
}

// HasKeys reports whether all keys are present in the map.
func HasKeys(attrs pcommon.Map, keys ...string) bool {
	for _, k := range keys {
		if _, ok := attrs.Get(k); !ok {
			return false
		}
	}
	return true
}

// PresentKeys returns the subset of keys present in the map, in the given order.
func PresentKeys(attrs pcommon.Map, keys ...string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if _, ok := attrs.Get(k); ok {
			out = append(out, k)
		}
	}
	return out
}
