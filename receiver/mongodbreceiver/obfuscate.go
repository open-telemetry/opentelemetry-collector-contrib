// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
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
		},
	}))
}

func (o *obfuscator) obfuscateMongoDBString(command string) string {
	return (*obfuscate.Obfuscator)(o).ObfuscateMongoDBString(command)
}

func cleanCommand(command bson.D) bson.D {
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
