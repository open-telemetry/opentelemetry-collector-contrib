// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
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
