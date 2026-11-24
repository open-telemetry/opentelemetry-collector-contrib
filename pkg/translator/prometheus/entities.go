// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"
)

// EntityInfo represents the OpenTelemetry entity metadata carried by an
// otel_entity (otel_entity_info) Prometheus info metric.
type EntityInfo struct {
	Type            string
	SchemaURL       string
	IDKeys          []string
	DescriptionKeys []string
}

// EncodeEntityKeys encodes attribute keys as a compact RFC 8259 JSON array of
// strings without HTML escaping, matching the OpenTelemetry common specification
// for representing arrays in non-OTLP protocols.
func EncodeEntityKeys(keys []string) (string, error) {
	if keys == nil {
		keys = []string{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(keys); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// ParseEntityInfo parses and syntactically validates the otel_entity_* labels
// from a Prometheus otel_entity_info metric. It returns false if required labels
// are missing/empty or if key lists are not valid JSON arrays of non-empty strings.
func ParseEntityInfo(entityType, schemaURL, rawIDKeys, rawDescKeys string) (EntityInfo, bool) {
	if entityType == "" {
		return EntityInfo{}, false
	}
	idKeys, ok := decodeEntityKeyList(rawIDKeys, false)
	if !ok {
		return EntityInfo{}, false
	}
	var descKeys []string
	if rawDescKeys != "" {
		descKeys, ok = decodeEntityKeyList(rawDescKeys, true)
		if !ok {
			return EntityInfo{}, false
		}
	}
	return EntityInfo{
		Type:            entityType,
		SchemaURL:       schemaURL,
		IDKeys:          idKeys,
		DescriptionKeys: descKeys,
	}, true
}

func decodeEntityKeyList(raw string, allowEmpty bool) ([]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed[0] != '[' {
		return nil, false
	}
	var parsed []string
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, false
	}
	if !allowEmpty && len(parsed) == 0 {
		return nil, false
	}
	seen := make(map[string]struct{}, len(parsed))
	deduped := make([]string, 0, len(parsed))
	for _, k := range parsed {
		if k == "" {
			return nil, false
		}
		if _, exists := seen[k]; exists {
			continue
		}
		seen[k] = struct{}{}
		deduped = append(deduped, k)
	}
	return deduped, true
}

type reconciledEntity struct {
	entityType string
	schemaURL  string
	idKeys     []string // deduplicated, sorted for set comparison
	idKeySet   map[string]struct{}
	descKeys   []string
	descKeySet map[string]struct{}
}

func reconcileEntities(candidates []EntityInfo, attrs pcommon.Map, skipEmptyValues bool) []reconciledEntity {
	byType := make(map[string]int, len(candidates))
	result := make([]reconciledEntity, 0, len(candidates))

	for _, c := range candidates {
		if c.Type == "" || len(c.IDKeys) == 0 {
			continue
		}
		idKeySet := make(map[string]struct{}, len(c.IDKeys))
		idKeys := make([]string, 0, len(c.IDKeys))
		valid := true
		for _, k := range c.IDKeys {
			if k == "" {
				valid = false
				break
			}
			v, ok := attrs.Get(k)
			if !ok || (skipEmptyValues && v.AsString() == "") {
				valid = false
				break
			}
			if _, exists := idKeySet[k]; exists {
				continue
			}
			idKeySet[k] = struct{}{}
			idKeys = append(idKeys, k)
		}
		if !valid || len(idKeys) == 0 {
			continue
		}
		slices.Sort(idKeys)

		descKeySet := make(map[string]struct{}, len(c.DescriptionKeys))
		descKeys := make([]string, 0, len(c.DescriptionKeys))
		for _, k := range c.DescriptionKeys {
			if k == "" {
				continue
			}
			if _, inID := idKeySet[k]; inID {
				continue
			}
			v, ok := attrs.Get(k)
			if !ok || (skipEmptyValues && v.AsString() == "") {
				continue
			}
			if _, exists := descKeySet[k]; exists {
				continue
			}
			descKeySet[k] = struct{}{}
			descKeys = append(descKeys, k)
		}

		if idx, exists := byType[c.Type]; exists {
			existing := &result[idx]
			if existing.schemaURL == c.SchemaURL && slices.Equal(existing.idKeys, idKeys) {
				for _, k := range descKeys {
					if _, has := existing.descKeySet[k]; !has {
						existing.descKeySet[k] = struct{}{}
						existing.descKeys = append(existing.descKeys, k)
					}
				}
				continue
			}
			result[idx] = reconciledEntity{
				entityType: c.Type,
				schemaURL:  c.SchemaURL,
				idKeys:     idKeys,
				idKeySet:   idKeySet,
				descKeys:   descKeys,
				descKeySet: descKeySet,
			}
			continue
		}

		byType[c.Type] = len(result)
		result = append(result, reconciledEntity{
			entityType: c.Type,
			schemaURL:  c.SchemaURL,
			idKeys:     idKeys,
			idKeySet:   idKeySet,
			descKeys:   descKeys,
			descKeySet: descKeySet,
		})
	}

	return result
}

// BuildEntityInfos converts valid Resource.entity_refs into sorted, deduplicated
// EntityInfo entries ready for export as otel_entity_info metrics. Key names are
// translated using translateKey (matching the label-name translation used for
// target_info). If translateKey is nil, keys are kept as-is.
func BuildEntityInfos(resource pcommon.Resource, translateKey func(string) (string, error)) ([]EntityInfo, error) {
	refs := entity.ResourceEntityRefs(resource)
	if refs.Len() == 0 {
		return nil, nil
	}

	candidates := make([]EntityInfo, 0, refs.Len())
	for i := 0; i < refs.Len(); i++ {
		er := refs.At(i)
		idSlice := er.IdKeys()
		idKeys := make([]string, idSlice.Len())
		for j := 0; j < idSlice.Len(); j++ {
			idKeys[j] = idSlice.At(j)
		}
		descSlice := er.DescriptionKeys()
		descKeys := make([]string, descSlice.Len())
		for j := 0; j < descSlice.Len(); j++ {
			descKeys[j] = descSlice.At(j)
		}
		candidates = append(candidates, EntityInfo{
			Type:            er.Type(),
			SchemaURL:       er.SchemaUrl(),
			IDKeys:          idKeys,
			DescriptionKeys: descKeys,
		})
	}

	// Prometheus treats empty label values as absent, so skip attributes whose
	// string representation is empty to avoid referencing keys omitted from target_info.
	reconciled := reconcileEntities(candidates, resource.Attributes(), true)
	if len(reconciled) == 0 {
		return nil, nil
	}

	out := make([]EntityInfo, 0, len(reconciled))
	for _, ent := range reconciled {
		translatedIDSet := make(map[string]struct{}, len(ent.idKeys))
		translatedIDKeys := make([]string, 0, len(ent.idKeys))
		for _, k := range ent.idKeys {
			tk := k
			if translateKey != nil {
				var err error
				tk, err = translateKey(k)
				if err != nil {
					return nil, err
				}
			}
			if _, exists := translatedIDSet[tk]; exists {
				continue
			}
			translatedIDSet[tk] = struct{}{}
			translatedIDKeys = append(translatedIDKeys, tk)
		}
		slices.Sort(translatedIDKeys)

		translatedDescSet := make(map[string]struct{}, len(ent.descKeys))
		translatedDescKeys := make([]string, 0, len(ent.descKeys))
		for _, k := range ent.descKeys {
			tk := k
			if translateKey != nil {
				var err error
				tk, err = translateKey(k)
				if err != nil {
					return nil, err
				}
			}
			if _, inID := translatedIDSet[tk]; inID {
				continue
			}
			if _, exists := translatedDescSet[tk]; exists {
				continue
			}
			translatedDescSet[tk] = struct{}{}
			translatedDescKeys = append(translatedDescKeys, tk)
		}
		slices.Sort(translatedDescKeys)

		out = append(out, EntityInfo{
			Type:            ent.entityType,
			SchemaURL:       ent.schemaURL,
			IDKeys:          translatedIDKeys,
			DescriptionKeys: translatedDescKeys,
		})
	}

	slices.SortFunc(out, func(a, b EntityInfo) int {
		return strings.Compare(a.Type, b.Type)
	})
	return out, nil
}

// ApplyEntityInfos validates and reconciles parsed otel_entity_info entries
// against resource.Attributes() and replaces resource's EntityRefs with the
// resulting entries.
func ApplyEntityInfos(resource pcommon.Resource, infos []EntityInfo) {
	reconciled := reconcileEntities(infos, resource.Attributes(), false)
	refs := entity.ResourceEntityRefs(resource)
	refs.RemoveIf(func(entity.EntityRef) bool { return true })
	for _, ent := range reconciled {
		slices.Sort(ent.descKeys)
		er := refs.AppendEmpty()
		er.SetType(ent.entityType)
		if ent.schemaURL != "" {
			er.SetSchemaUrl(ent.schemaURL)
		}
		for _, k := range ent.idKeys {
			er.IdKeys().Append(k)
		}
		for _, k := range ent.descKeys {
			er.DescriptionKeys().Append(k)
		}
	}
}
