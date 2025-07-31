// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	keysToRemoveFromExplainPlan = map[string]bool{
		"serverInfo":          true,
		"serverParameters":    true,
		"command":             true,
		"ok":                  true,
		"$clusterTime":        true,
		"operationTime":       true,
		"$configServerState":  true,
		"lastCommittedOpTime": true,
		"$gleStats":           true,
	}
	keysToCleanFromCommand = map[string]bool{
		"comment":      true,
		"lsid":         true,
		"$clusterTime": true,
	}
	keysToRemoveFromCommandForExplain = map[string]bool{
		"$db":                    true,
		"readConcern":            true,
		"writeConcern":           true,
		"needsMerge":             true,
		"fromMongos":             true,
		"let":                    true,
		"mayBypassWriteBlocking": true,
	}
)

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscate.Config{
		Mongo: obfuscate.JSONConfig{
			Enabled: true,
		},
	}))
}

func (o *obfuscator) obfuscateSQLString(sql string) string {
	return (*obfuscate.Obfuscator)(o).ObfuscateMongoDBString(sql)
}

// generateQuerySignature creates a unique signature for the query
func generateQuerySignature(obfuscatedStatement string) string {
	hash := sha256.Sum256([]byte(obfuscatedStatement))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter signature
}

// prepareCommandForExplain prepares a command to be explainable by removing
func prepareCommandForExplain(command bson.D) bson.D {
	explainableCommand := make(bson.D, len(command))

	finalLen := 0

	for _, v := range command {
		if v.Key == "" {
			continue
		}
		if !keysToRemoveFromCommandForExplain[v.Key] {
			explainableCommand[finalLen] = v
			finalLen++
		}
	}
	cleaned := make(bson.D, finalLen)
	copy(cleaned, command)
	return cleaned
}

// cleanExplainPlan removes unnecessary keys from the explain plan result
func cleanExplainPlan(explainResult bson.M) bson.M {
	cleaned := make(bson.M)

	for k, v := range explainResult {
		if !keysToRemoveFromExplainPlan[k] {
			cleaned[k] = v
		}
	}

	return cleaned
}

func cleanCommand(command bson.D) bson.D {
	// Parse the JSON statement back to a map for proper obfuscation
	commandCopied := make(bson.D, len(command))

	finalLen := 0
	for _, v := range command {
		if v.Key == "" {
			continue
		}
		if _, ok := keysToCleanFromCommand[v.Key]; ok {
			continue
		}
		commandCopied[finalLen] = v
		finalLen++
	}
	cleaned := make(bson.D, finalLen)
	copy(cleaned, commandCopied)
	return cleaned
}

// obfuscateLiterals recursively obfuscates literal values while preserving structure
func obfuscateLiterals(value any) any {
	switch v := value.(type) {
	case bson.M:
		obfuscated := make(bson.M)
		for k, val := range v {
			obfuscated[k] = obfuscateLiterals(val)
		}
		return obfuscated
	case map[string]any:
		obfuscated := make(map[string]any)
		for k, val := range v {
			obfuscated[k] = obfuscateLiterals(val)
		}
		return obfuscated
	case bson.A:
		obfuscated := make(bson.A, len(v))
		for i, val := range v {
			obfuscated[i] = obfuscateLiterals(val)
		}
		return obfuscated
	case []any:
		obfuscated := make([]any, len(v))
		for i, val := range v {
			obfuscated[i] = obfuscateLiterals(val)
		}
		return obfuscated
	case string, int, int32, int64, float32, float64, bool:
		// Replace literal values with placeholder
		return "?"
	default:
		return value
	}
}

// obfuscateExplainPlan obfuscates sensitive data in explain plans
func obfuscateExplainPlan(plan any) any {
	switch p := plan.(type) {
	case bson.M:
		obfuscated := make(bson.M)
		for key, value := range p {
			if key == "filter" || key == "parsedQuery" || key == "indexBounds" {
				obfuscated[key] = obfuscateLiterals(value)
			} else {
				obfuscated[key] = obfuscateExplainPlan(value)
			}
		}
		return obfuscated
	case map[string]any:
		obfuscated := make(map[string]any)
		for key, value := range p {
			if key == "filter" || key == "parsedQuery" || key == "indexBounds" {
				obfuscated[key] = obfuscateLiterals(value)
			} else {
				obfuscated[key] = obfuscateExplainPlan(value)
			}
		}
		return obfuscated
	case bson.A:
		obfuscated := make(bson.A, len(p))
		for i, value := range p {
			obfuscated[i] = obfuscateExplainPlan(value)
		}
		return obfuscated
	case []any:
		obfuscated := make([]any, len(p))
		for i, value := range p {
			obfuscated[i] = obfuscateExplainPlan(value)
		}
		return obfuscated
	default:
		return p
	}
}
