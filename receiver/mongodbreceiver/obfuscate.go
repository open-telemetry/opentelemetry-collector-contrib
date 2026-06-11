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

func (o *obfuscator) obfuscateCommand(command bson.D) string {
	serialized, err := bson.MarshalExtJSON(command, false, false)
	if err != nil {
		return ""
	}
	return o.obfuscateMongoDBString(string(serialized))
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
