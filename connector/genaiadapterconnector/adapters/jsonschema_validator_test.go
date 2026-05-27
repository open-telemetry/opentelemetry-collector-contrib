// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Lightweight JSON Schema validator for testing.
//
// Implements a subset of the JSON Schema specification (Draft 2020-12):
// https://json-schema.org/specification
package adapters

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	schemaCache   = make(map[string]map[string]any)
	schemaCacheMu sync.Mutex
)

func FetchJSONSchema(t *testing.T, url string) map[string]any {
	t.Helper()
	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()

	if s, ok := schemaCache[url]; ok {
		return s
	}

	resp, err := http.Get(url) //nolint:gosec
	require.NoError(t, err, "failed to fetch schema from %s", url)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "unexpected status fetching %s", url)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(body, &schema))

	schemaCache[url] = schema
	return schema
}

func ResolveDef(schema map[string]any, ref string) (map[string]any, bool) {
	// see: https://json-schema.org/draft/2020-12/json-schema-core#section-8.2.3.1
	if len(ref) < 9 || ref[:8] != "#/$defs/" {
		return nil, false
	}
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		return nil, false
	}
	def, ok := defs[ref[8:]].(map[string]any)
	return def, ok
}

func ValidateArrayItems(t *testing.T, items []map[string]any, schema map[string]any, context string) {
	// see: https://json-schema.org/draft/2020-12/json-schema-core#section-10.3.1.2
	t.Helper()

	if typ, ok := schema["type"].(string); ok {
		assert.Equal(t, "array", typ, "%s: expected array type", context)
	}

	var itemDef map[string]any
	if itemsSchema, ok := schema["items"].(map[string]any); ok {
		if ref, ok := itemsSchema["$ref"].(string); ok {
			def, found := ResolveDef(schema, ref)
			require.True(t, found, "could not resolve item $ref %s", ref)
			itemDef = def
		}
	}

	for i, item := range items {
		if itemDef != nil {
			ValidateObjectAgainstDef(t, item, itemDef, schema, fmt.Sprintf("%s[%d]", context, i))
		}
	}
}

func ValidateObjectAgainstDef(t *testing.T, obj, def, schema map[string]any, context string) {
	t.Helper()

	// see: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.5.3
	if required, ok := def["required"].([]any); ok {
		ValidateRequired(t, obj, required, context)
	}

	// see: https://json-schema.org/draft/2020-12/json-schema-core#section-10.3.2.1
	properties, _ := def["properties"].(map[string]any)
	for key, val := range obj {
		propSchema, hasProp := properties[key].(map[string]any)
		if !hasProp {
			continue
		}

		fieldCtx := fmt.Sprintf("%s.%s", context, key)

		// see: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1.1
		if typ, ok := propSchema["type"].(string); ok {
			ValidateType(t, val, typ, fieldCtx)
		}

		// see: https://json-schema.org/draft/2020-12/json-schema-core#section-8.2.3.1
		if ref, ok := propSchema["$ref"].(string); ok {
			if refDef, found := ResolveDef(schema, ref); found {
				if enum, ok := refDef["enum"].([]any); ok {
					ValidateEnum(t, val, enum, fieldCtx)
				}
			}
		}

		// see: https://json-schema.org/draft/2020-12/json-schema-core#section-10.2.1.2
		if anyOf, ok := propSchema["anyOf"].([]any); ok {
			for _, option := range anyOf {
				optMap, _ := option.(map[string]any)
				if ref, ok := optMap["$ref"].(string); ok {
					if refDef, found := ResolveDef(schema, ref); found {
						if enum, ok := refDef["enum"].([]any); ok {
							ValidateEnum(t, val, enum, fieldCtx)
						}
					}
				}
			}
		}

		// see: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1.3
		if constVal, ok := propSchema["const"]; ok {
			assert.Equal(t, constVal, val, "%s: expected const %v", fieldCtx, constVal)
		}
	}
}

func ValidateRequired(t *testing.T, obj map[string]any, required []any, context string) {
	// see: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.5.3
	t.Helper()
	for _, r := range required {
		key, _ := r.(string)
		_, exists := obj[key]
		assert.True(t, exists, "%s: missing required field %q", context, key)
	}
}

func ValidateType(t *testing.T, value any, expectedType, context string) {
	// see: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1.1
	t.Helper()
	switch expectedType {
	case "string":
		_, ok := value.(string)
		assert.True(t, ok, "%s: expected string, got %T", context, value)
	case "array":
		_, ok := value.([]any)
		assert.True(t, ok, "%s: expected array, got %T", context, value)
	case "object":
		_, ok := value.(map[string]any)
		assert.True(t, ok, "%s: expected object, got %T", context, value)
	}
}

func ValidateEnum(t *testing.T, value any, enum []any, context string) {
	// see: https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1.2
	t.Helper()
	for _, e := range enum {
		if value == e {
			return
		}
	}
	assert.Fail(t, fmt.Sprintf("%s: value %q not in enum %v", context, value, enum))
}
