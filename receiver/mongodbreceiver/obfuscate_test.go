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
		{Key: "lsid", Value: bson.M{"id": "session-1"}},
		{Key: "$clusterTime", Value: bson.M{"clusterTime": "time"}},
	}

	cleanedCommand := cleanCommand(command)
	require.Len(t, cleanedCommand, 2)
	require.Equal(t, bson.D{
		{Key: "find", Value: "users"},
		{Key: "filter", Value: bson.M{"name": "test", "age": 30}},
	}, cleanedCommand)

	obfuscated := o.obfuscateCommand(cleanedCommand)
	require.Contains(t, obfuscated, "find")
	require.NotContains(t, obfuscated, "comment")
	require.NotContains(t, obfuscated, "lsid")
	require.NotContains(t, obfuscated, "$clusterTime")
	// "users" is the value of the "find" key, which is in KeepValues — preserved intentionally.
	require.Contains(t, obfuscated, "users")
	// Filter values are still obfuscated.
	require.NotContains(t, obfuscated, "test")
	require.NotContains(t, obfuscated, "30")
	require.NotContains(t, obfuscated, "session-1")
	require.NotContains(t, obfuscated, "time")
}

func TestObfuscateCommandUsesRelaxedExtendedJSON(t *testing.T) {
	o := newObfuscator()

	command := bson.D{
		{Key: "getMore", Value: int64(4296433915628356331)},
		{Key: "collection", Value: "orders"},
		{Key: "$db", Value: "sample_business"},
	}

	obfuscated := o.obfuscateCommand(command)
	require.Contains(t, obfuscated, `"getMore":"?"`)
	require.NotContains(t, obfuscated, "$numberLong")
}
