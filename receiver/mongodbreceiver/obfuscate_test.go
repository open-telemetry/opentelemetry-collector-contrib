// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestObfuscateCommand(t *testing.T) {
	o := newObfuscator()

	command := bson.D{
		{Key: "find", Value: "users"},
		{Key: "filter", Value: bson.M{"name": "test", "age": 30}},
		{Key: "comment", Value: "test query"},
	}

	cleanedCommand := cleanCommand(command)
	obfuscated := o.obfuscateMongoDBString(cleanedCommand.String())
	require.Contains(t, obfuscated, "find")
	require.NotContains(t, obfuscated, "users")
	require.NotContains(t, obfuscated, "test")
	require.NotContains(t, obfuscated, "30")
}
