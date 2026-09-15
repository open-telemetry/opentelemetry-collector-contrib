// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"encoding/json"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var keysToCleanFromCommand = map[string]bool{
	"comment":      true,
	"lsid":         true,
	"$clusterTime": true,
}

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscate.Config{
		Mongo: obfuscate.JSONConfig{
			Enabled: true,
			KeepValues: []string{
				"$db",
				"aggregate",
				"collection",
				"count",
				"delete",
				"distinct",
				"find",
				"findAndModify",
				"insert",
				"update",
			},
		},
	}))
}

func (o *obfuscator) obfuscateMongoDBString(command string) string {
	return (*obfuscate.Obfuscator)(o).ObfuscateMongoDBString(command)
}

func (o *obfuscator) obfuscateCommand(command bson.D) (string, error) {
	serialized, err := bson.MarshalExtJSON(command, false, false)
	if err != nil {
		return "", err
	}
	return o.obfuscateMongoDBString(string(serialized)), nil
}

func cleanCommand(command bson.D) bson.D {
	cleaned := make(bson.D, 0, len(command))
	for _, v := range command {
		if v.Key == "" {
			continue
		}
		if _, ok := keysToCleanFromCommand[v.Key]; ok {
			continue
		}
		cleaned = append(cleaned, v)
	}
	return cleaned
}

// asMap normalises map[string]any, bson.M, and bson.D into map[string]any.
func asMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case bson.M:
		return m
	case bson.D:
		out := make(map[string]any, len(m))
		for _, e := range m {
			out[e.Key] = e.Value
		}
		return out
	}
	return nil
}

// asSlice normalises both []any and bson.A into []any.
func asSlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case bson.A:
		return s
	}
	return nil
}

// obfuscateExplainPlan recursively walks an explain result and replaces literal
// values in "filter", "parsedQuery", and "indexBounds" with "?".
func obfuscateExplainPlan(v any) any {
	if m := asMap(v); m != nil {
		out := make(map[string]any, len(m))
		for k, child := range m {
			switch k {
			case "filter", "parsedQuery", "indexBounds", "slotBasedPlan":
				out[k] = obfuscateLiterals(child)
			default:
				out[k] = obfuscateExplainPlan(child)
			}
		}
		return out
	}
	if s := asSlice(v); s != nil {
		out := make([]any, len(s))
		for i, item := range s {
			out[i] = obfuscateExplainPlan(item)
		}
		return out
	}
	return v
}

// obfuscateLiterals replaces all leaf values with "?", preserving only map and
// slice structure. nil is kept as-is
func obfuscateLiterals(v any) any {
	if v == nil {
		return nil
	}
	if m := asMap(v); m != nil {
		out := make(map[string]any, len(m))
		for k, child := range m {
			out[k] = obfuscateLiterals(child)
		}
		return out
	}
	if s := asSlice(v); s != nil {
		out := make([]any, len(s))
		for i, item := range s {
			out[i] = obfuscateLiterals(item)
		}
		return out
	}
	return "?"
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
