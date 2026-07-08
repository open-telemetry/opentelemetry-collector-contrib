// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"encoding/json"
	"fmt"
	"hash/fnv"

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

// commandKeysToStrip are fields removed before running explain.
var commandKeysToStrip = map[string]struct{}{
	"$clusterTime":           {},
	"$db":                    {},
	"comment":                {},
	"fromMongos":             {},
	"lsid":                   {},
	"mayBypassWriteBlocking": {},
	"needsMerge":             {},
	"readConcern":            {},
	"txnNumber":              {},
	"writeConcern":           {},
}

// unexplainableCommands are commands that cannot be wrapped in an explain.
var unexplainableCommands = map[string]struct{}{
	"buildInfo":        {},
	"collStats":        {},
	"createIndexes":    {},
	"dbStats":          {},
	"delete":           {},
	"explain":          {},
	"getLog":           {},
	"getMore":          {},
	"insert":           {},
	"isMaster":         {},
	"hello":            {},
	"listCollections":  {},
	"listDatabases":    {},
	"listIndexes":      {},
	"ping":             {},
	"profile":          {},
	"replSetGetStatus": {},
	"serverStatus":     {},
	"shardCollection":  {},
	"top":              {},
	"update":           {},
}

func stripKeys(cmd bson.D, strip map[string]struct{}) bson.D {
	out := make(bson.D, 0, len(cmd))
	for _, e := range cmd {
		if e.Key == "" {
			continue
		}
		if _, skip := strip[e.Key]; !skip {
			out = append(out, e)
		}
	}
	return out
}

// querySignature returns a stable FNV-64a hex string of the obfuscated command
func querySignature(obfuscated string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(obfuscated))
	return fmt.Sprintf("%016x", h.Sum64())
}

// isExplainable reports whether a bson.D command can be passed to the explain command.
func isExplainable(cmd bson.D) bool {
	for _, e := range cmd {
		if _, unexplainable := unexplainableCommands[e.Key]; unexplainable {
			return false
		}
	}
	return len(cmd) > 0
}

// prepareForExplainCleaned wraps an already-stripped command for explain.
// For aggregate commands it adds cursor:{} which MongoDB requires.
func prepareForExplainCleaned(cmd bson.D) bson.D {
	for _, e := range cmd {
		if e.Key == "aggregate" {
			for _, e2 := range cmd {
				if e2.Key == "cursor" {
					return cmd
				}
			}
			return append(cmd, bson.E{Key: "cursor", Value: bson.D{}})
		}
	}
	return cmd
}

// cleanExplainResult removes server metadata fields from an explain result.
func cleanExplainResult(result map[string]any) map[string]any {
	drop := map[string]struct{}{
		"serverInfo":       {},
		"serverParameters": {},
		"command":          {},
		"ok":               {},
		"$clusterTime":     {},
		"operationTime":    {},
	}
	out := make(map[string]any, len(result))
	for k, v := range result {
		if _, skip := drop[k]; !skip {
			out[k] = v
		}
	}
	return out
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
// values in "filter", "parsedQuery", and "indexBounds" with "?"
func obfuscateExplainPlan(v any) any {
	if m := asMap(v); m != nil {
		out := make(map[string]any, len(m))
		for k, child := range m {
			switch k {
			case "filter", "parsedQuery", "indexBounds":
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
