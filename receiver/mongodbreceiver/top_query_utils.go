// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"fmt"
	"hash/fnv"

	"go.mongodb.org/mongo-driver/v2/bson"
)

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

// unexplainableOps are system.profile op values whose commands cannot be
// wrapped in an explain.
var unexplainableOps = map[string]struct{}{
	"insert":      {},
	"update":      {},
	"remove":      {},
	"getmore":     {},
	"killcursors": {},
	"none":        {},
	"":            {},
}

// isExplainable reports whether a profile entry's command can be passed to
// the explain command.
func isExplainable(op string, cmd bson.D) bool {
	if _, skip := unexplainableOps[op]; skip {
		return false
	}
	if len(cmd) == 0 {
		return false
	}
	// findAndModify legitimately has "update" (or "remove") as inner clauses,
	// which would otherwise trip the all-keys check below. MongoDB dispatches
	// explain by the first key, so as long as the verb (findAndModify) is
	// explainable, the nested clauses are safe.
	if cmd[0].Key == "findAndModify" {
		return true
	}
	for _, e := range cmd {
		if _, unexplainable := unexplainableCommands[e.Key]; unexplainable {
			return false
		}
	}
	return true
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
