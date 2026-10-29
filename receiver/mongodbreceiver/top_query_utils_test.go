// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestQuerySignature_StableForSameInput(t *testing.T) {
	first := querySignature("foo")
	second := querySignature("foo")
	require.Equal(t, first, second)
	require.NotEqual(t, querySignature("foo"), querySignature("bar"))
	require.Len(t, querySignature("anything"), 16)
}

func TestIsExplainable(t *testing.T) {
	// op="query" (find) and op="command" (aggregate) are candidates for explain.
	require.True(t, isExplainable("query", bson.D{{Key: "find", Value: "users"}}))
	require.True(t, isExplainable("command", bson.D{{Key: "aggregate", Value: "users"}, {Key: "pipeline", Value: bson.A{}}}))

	// Unexplainable ops are skipped regardless of the command payload.
	// Write ops in system.profile appear as {q,u,multi,upsert} or {q,limit},
	// not as runnable commands — the op-based filter catches them first.
	require.False(t, isExplainable("update", bson.D{{Key: "q", Value: bson.D{}}, {Key: "u", Value: bson.D{}}}))
	require.False(t, isExplainable("remove", bson.D{{Key: "q", Value: bson.D{}}, {Key: "limit", Value: int32(0)}}))
	require.False(t, isExplainable("insert", bson.D{{Key: "insert", Value: "users"}}))
	require.False(t, isExplainable("getmore", bson.D{{Key: "getMore", Value: int64(1)}}))
	require.False(t, isExplainable("killcursors", bson.D{{Key: "killCursors", Value: "users"}}))
	require.False(t, isExplainable("none", bson.D{{Key: "find", Value: "users"}}))
	require.False(t, isExplainable("", bson.D{{Key: "find", Value: "users"}}))

	// op="command" but the command itself is unexplainable.
	require.False(t, isExplainable("command", bson.D{{Key: "update", Value: "users"}}))
	require.False(t, isExplainable("command", bson.D{{Key: "delete", Value: "users"}}))
	require.False(t, isExplainable("command", bson.D{{Key: "explain", Value: bson.D{}}}))
	require.False(t, isExplainable("command", bson.D{{Key: "listCollections", Value: int32(1)}}))

	// Empty command is never explainable.
	require.False(t, isExplainable("query", bson.D{}))

	// findAndModify legitimately nests "update" or "remove" clauses. MongoDB
	// dispatches explain by the first key, so these must be treated as
	// explainable despite the inner keys matching unexplainableCommands.
	require.True(t, isExplainable("command", bson.D{
		{Key: "findAndModify", Value: "users"},
		{Key: "query", Value: bson.D{{Key: "status", Value: "pending"}}},
		{Key: "update", Value: bson.D{{Key: "$set", Value: bson.D{{Key: "status", Value: "done"}}}}},
	}))
	require.True(t, isExplainable("command", bson.D{
		{Key: "findAndModify", Value: "users"},
		{Key: "query", Value: bson.D{{Key: "status", Value: "pending"}}},
		{Key: "remove", Value: true},
	}))
}

func TestStripKeys_StripsAllDriverMetadata(t *testing.T) {
	cmd := bson.D{
		{Key: "aggregate", Value: "users"},
		{Key: "lsid", Value: bson.D{}},
		{Key: "$clusterTime", Value: bson.D{}},
		{Key: "$db", Value: "appdb"},
		{Key: "readConcern", Value: bson.D{}},
		{Key: "writeConcern", Value: bson.D{}},
		{Key: "txnNumber", Value: int64(1)},
		{Key: "pipeline", Value: bson.A{}},
	}
	out := stripKeys(cmd, commandKeysToStrip)
	keys := keysOf(out)
	require.Contains(t, keys, "aggregate")
	require.Contains(t, keys, "pipeline")
	require.NotContains(t, keys, "lsid")
	require.NotContains(t, keys, "$clusterTime")
	require.NotContains(t, keys, "$db")
	require.NotContains(t, keys, "readConcern")
	require.NotContains(t, keys, "writeConcern")
	require.NotContains(t, keys, "txnNumber")
}

func TestPrepareForExplainCleaned_AddsCursorForAggregate(t *testing.T) {
	cmd := bson.D{
		{Key: "aggregate", Value: "users"},
		{Key: "pipeline", Value: bson.A{}},
	}
	out := prepareForExplainCleaned(cmd)
	require.Contains(t, keysOf(out), "cursor")
}

func TestPrepareForExplainCleaned_KeepsExistingCursor(t *testing.T) {
	cmd := bson.D{
		{Key: "aggregate", Value: "users"},
		{Key: "pipeline", Value: bson.A{}},
		{Key: "cursor", Value: bson.D{{Key: "batchSize", Value: 100}}},
	}
	out := prepareForExplainCleaned(cmd)
	cursorCount := 0
	for _, e := range out {
		if e.Key == "cursor" {
			cursorCount++
		}
	}
	require.Equal(t, 1, cursorCount)
}

func TestPrepareForExplainCleaned_DoesNotAddCursorForFind(t *testing.T) {
	cmd := bson.D{{Key: "find", Value: "users"}}
	out := prepareForExplainCleaned(cmd)
	require.NotContains(t, keysOf(out), "cursor")
}

func TestCleanExplainResult_DropsServerMetadata(t *testing.T) {
	in := map[string]any{
		"queryPlanner":  map[string]any{"winningPlan": "x"},
		"serverInfo":    map[string]any{"host": "h"},
		"ok":            float64(1),
		"$clusterTime":  map[string]any{"t": 1},
		"operationTime": int64(123),
		"command":       map[string]any{"find": "x"},
	}
	out := cleanExplainResult(in)
	require.Contains(t, out, "queryPlanner")
	require.NotContains(t, out, "serverInfo")
	require.NotContains(t, out, "ok")
	require.NotContains(t, out, "$clusterTime")
	require.NotContains(t, out, "operationTime")
	require.NotContains(t, out, "command")
}
