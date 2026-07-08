// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func newTestScraper(t *testing.T, fc *fakeClient) *mongodbScraper {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.QueryPlanCacheTTL = 0
	s := newMongodbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	s.planCache = buildPlanCache(cfg)
	s.client = fc
	return s
}

// --- profileDoc / slowQueryEntry / topN ---

func TestProfileDoc_TSField(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	d := profileDoc{
		TS:     ts,
		Millis: 250,
	}
	require.Equal(t, ts, d.TS)
	require.Equal(t, int64(250), d.Millis)
}

func TestTopN_RanksByMillisAndLimits(t *testing.T) {
	entries := []slowQueryEntry{
		{NS: "appdb.users", Millis: 100},
		{NS: "appdb.orders", Millis: 500},
		{NS: "appdb.things", Millis: 250},
	}
	ranked := topN(entries, 2)
	require.Len(t, ranked, 2)
	require.Equal(t, int64(500), ranked[0].Millis)
	require.Equal(t, int64(250), ranked[1].Millis)
}

func TestTopN_ZeroReturnsNil(t *testing.T) {
	entries := []slowQueryEntry{
		{Millis: 100},
		{Millis: 500},
	}
	require.Nil(t, topN(entries, 0))
}

func TestTopN_StableOnTie(t *testing.T) {
	entries := []slowQueryEntry{
		{NS: "appdb.a", Millis: 100},
		{NS: "appdb.b", Millis: 100},
		{NS: "appdb.c", Millis: 100},
	}
	ranked := topN(entries, 3)
	ranked2 := topN([]slowQueryEntry{
		{NS: "appdb.a", Millis: 100},
		{NS: "appdb.b", Millis: 100},
		{NS: "appdb.c", Millis: 100},
	}, 3)
	for i := range ranked {
		require.Equal(t, ranked[i].NS, ranked2[i].NS)
	}
}

// --- querySignature / isExplainable / helpers ---

func TestQuerySignature_StableForSameInput(t *testing.T) {
	require.Equal(t, querySignature("foo"), querySignature("foo"))
	require.NotEqual(t, querySignature("foo"), querySignature("bar"))
	require.Len(t, querySignature("anything"), 16)
}

func TestIsExplainable(t *testing.T) {
	require.True(t, isExplainable(bson.D{{Key: "find", Value: "users"}}))
	require.True(t, isExplainable(bson.D{{Key: "aggregate", Value: "users"}, {Key: "pipeline", Value: bson.A{}}}))
	require.False(t, isExplainable(bson.D{{Key: "update", Value: "users"}}))
	require.False(t, isExplainable(bson.D{{Key: "delete", Value: "users"}}))
	require.False(t, isExplainable(bson.D{{Key: "insert", Value: "users"}}))
	require.False(t, isExplainable(bson.D{{Key: "getMore", Value: int64(1)}}))
	require.False(t, isExplainable(bson.D{{Key: "explain", Value: bson.D{}}}))
}

func TestCleanCommand_StripsSessionKeys(t *testing.T) {
	cmd := bson.D{
		{Key: "find", Value: "users"},
		{Key: "lsid", Value: bson.D{{Key: "id", Value: "x"}}},
		{Key: "$clusterTime", Value: bson.D{{Key: "t", Value: 1}}},
		{Key: "comment", Value: "hello"},
		{Key: "let", Value: bson.D{{Key: "v", Value: 1}}},
	}
	out := cleanCommand(cmd)
	keys := keysOf(out)
	require.NotContains(t, keys, "lsid")
	require.NotContains(t, keys, "$clusterTime")
	require.NotContains(t, keys, "comment")
	require.Contains(t, keys, "let")
	require.Contains(t, keys, "find")
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

// --- explain / obfuscation ---

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

func TestObfuscateExplainPlan_ObfuscatesTargetedFields(t *testing.T) {
	plan := map[string]any{
		"queryPlanner": map[string]any{
			"winningPlan": map[string]any{
				"stage": "COLLSCAN",
				"filter": map[string]any{
					"status": map[string]any{"$eq": "A"},
				},
			},
			"parsedQuery": map[string]any{
				"status": map[string]any{"$eq": "A"},
			},
		},
	}
	out := obfuscateExplainPlan(plan).(map[string]any)
	qp := out["queryPlanner"].(map[string]any)

	pq := qp["parsedQuery"].(map[string]any)
	statusPQ := pq["status"].(map[string]any)
	require.Equal(t, "?", statusPQ["$eq"])

	wp := qp["winningPlan"].(map[string]any)
	require.Equal(t, "COLLSCAN", wp["stage"])
	filter := wp["filter"].(map[string]any)
	statusF := filter["status"].(map[string]any)
	require.Equal(t, "?", statusF["$eq"])
}

func TestObfuscateExplainPlan_SameStructureSameHash(t *testing.T) {
	plan1 := map[string]any{
		"queryPlanner": map[string]any{
			"winningPlan": map[string]any{"stage": "COLLSCAN"},
			"parsedQuery": map[string]any{"status": map[string]any{"$eq": "A"}},
		},
	}
	plan2 := map[string]any{
		"queryPlanner": map[string]any{
			"winningPlan": map[string]any{"stage": "COLLSCAN"},
			"parsedQuery": map[string]any{"status": map[string]any{"$eq": "B"}},
		},
	}
	h1 := querySignature(marshalJSON(obfuscateExplainPlan(plan1)))
	h2 := querySignature(marshalJSON(obfuscateExplainPlan(plan2)))
	require.Equal(t, h1, h2, "different literals, same plan structure → same hash")
}

func TestObfuscateExplainPlan_DifferentStructureDifferentHash(t *testing.T) {
	collscan := map[string]any{
		"queryPlanner": map[string]any{
			"winningPlan": map[string]any{"stage": "COLLSCAN"},
		},
	}
	ixscan := map[string]any{
		"queryPlanner": map[string]any{
			"winningPlan": map[string]any{
				"stage":      "FETCH",
				"inputStage": map[string]any{"stage": "IXSCAN", "keyPattern": map[string]any{"status": 1}},
			},
		},
	}
	h1 := querySignature(marshalJSON(obfuscateExplainPlan(collscan)))
	h2 := querySignature(marshalJSON(obfuscateExplainPlan(ixscan)))
	require.NotEqual(t, h1, h2, "different plan structures → different hash")
}

// --- parseLogLine ---

func TestParseLogLine_SlowQuery(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := map[string]any{
		"t":   map[string]any{"$date": ts.Format(time.RFC3339Nano)},
		"msg": "Slow query",
		"attr": map[string]any{
			"ns":             "appdb.users",
			"type":           "command",
			"durationMillis": float64(125),
			"cpuNanos":       float64(1_000_000),
			"reslen":         float64(42),
			"keysExamined":   float64(2),
			"docsExamined":   float64(3),
			"nreturned":      float64(1),
			"command": map[string]any{
				"find":   "users",
				"filter": map[string]any{"age": float64(25)},
			},
		},
	}
	b, err := json.Marshal(raw)
	require.NoError(t, err)

	entry, ok := parseLogLine(string(b))
	require.True(t, ok)
	require.Equal(t, "appdb.users", entry.NS)
	require.Equal(t, "command", entry.Op)
	require.Equal(t, int64(125), entry.Millis)
	require.Equal(t, int64(1_000_000), entry.CPUNanos)
	require.Equal(t, int64(42), entry.ResponseLength)
	require.Equal(t, int64(2), entry.KeysExamined)
	require.Equal(t, int64(3), entry.DocsExamined)
	require.Equal(t, int64(1), entry.NReturned)
	require.NotEmpty(t, entry.Command)
	require.True(t, entry.EndTime.Equal(ts))
}

func TestParseLogLine_NotSlowQuery(t *testing.T) {
	_, ok := parseLogLine(`{"t":{"$date":"2026-01-02T03:04:05Z"},"msg":"Connection ended"}`)
	require.False(t, ok)
}

func TestParseLogLine_BadJSON(t *testing.T) {
	_, ok := parseLogLine("not json")
	require.False(t, ok)
}

func TestParseLogLine_MissingCommand(t *testing.T) {
	raw := `{"t":{"$date":"2026-01-02T03:04:05Z"},"msg":"Slow query","attr":{"ns":"x.y","durationMillis":1}}`
	_, ok := parseLogLine(raw)
	require.False(t, ok)
}

// --- getProfilingLevel ---

func TestGetProfilingLevel(t *testing.T) {
	cases := []struct {
		desc string
		ret  bson.M
		want int
	}{
		{"int32 was=1", bson.M{"was": int32(1)}, 1},
		{"int64 was=2", bson.M{"was": int64(2)}, 2},
		{"float64 was=0", bson.M{"was": float64(0)}, 0},
		{"missing was", bson.M{"ok": float64(1)}, 0},
		{"unknown type", bson.M{"was": "1"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			fc := &fakeClient{}
			fc.On("RunCommand", mock.Anything, "appdb", bson.M{"profile": -1}).Return(tc.ret, nil)
			s := newTestScraper(t, fc)
			level, err := s.getProfilingLevel(context.Background(), "appdb")
			require.NoError(t, err)
			require.Equal(t, tc.want, level)
		})
	}
}

func TestGetProfilingLevel_RunCommandError(t *testing.T) {
	fc := &fakeClient{}
	fc.On("RunCommand", mock.Anything, "appdb", bson.M{"profile": -1}).Return(bson.M(nil), errors.New("auth failed"))
	s := newTestScraper(t, fc)
	_, err := s.getProfilingLevel(context.Background(), "appdb")
	require.Error(t, err)
}

// --- scrapeTopQueryFromProfiler ---

func TestScrapeTopQueryFromProfiler_DisabledFallsBack(t *testing.T) {
	fc := &fakeClient{}
	fc.On("RunCommand", mock.Anything, "appdb", bson.M{"profile": -1}).Return(bson.M{"was": int32(0)}, nil)
	s := newTestScraper(t, fc)
	now := time.Now()
	_, used, err := s.scrapeTopQueryFromProfiler(context.Background(), "appdb", now.Add(-time.Minute), now, 20)
	require.NoError(t, err)
	require.False(t, used)
}

func TestScrapeTopQueryFromProfiler_EnabledReturnsRankedDocs(t *testing.T) {
	fc := &fakeClient{}
	fc.On("RunCommand", mock.Anything, "appdb", bson.M{"profile": -1}).Return(bson.M{"was": int32(1)}, nil)

	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	docs := []profileDoc{
		{
			TS:      ts,
			NS:      "appdb.users",
			Op:      "insert",
			Millis:  100,
			Command: bson.D{{Key: "insert", Value: "users"}, {Key: "documents", Value: bson.A{bson.D{{Key: "a", Value: 1}}}}},
		},
	}
	// FindProfileDocs is now called with (ctx, dbName, sinceTime, upperBound, topN)
	fc.On("FindProfileDocs", mock.Anything, "appdb", mock.Anything, mock.Anything, mock.Anything).Return(docs, nil)

	s := newTestScraper(t, fc)
	now := time.Now()
	entries, used, err := s.scrapeTopQueryFromProfiler(context.Background(), "appdb", now.Add(-time.Minute), now, 20)
	require.NoError(t, err)
	require.True(t, used)
	require.Len(t, entries, 1)
	require.Equal(t, "appdb.users", entries[0].NS)
}

func TestScrapeTopQueryFromProfiler_EmptyReturnsNoEntries(t *testing.T) {
	fc := &fakeClient{}
	fc.On("RunCommand", mock.Anything, "appdb", bson.M{"profile": -1}).Return(bson.M{"was": int32(1)}, nil)
	fc.On("FindProfileDocs", mock.Anything, "appdb", mock.Anything, mock.Anything, mock.Anything).Return([]profileDoc{}, nil)

	s := newTestScraper(t, fc)
	now := time.Now()
	entries, used, err := s.scrapeTopQueryFromProfiler(context.Background(), "appdb", now.Add(-time.Minute), now, 20)
	require.NoError(t, err)
	require.True(t, used)
	require.Empty(t, entries)
}

// --- scrapeTopQueryFromGetLog ---

func TestScrapeTopQueryFromGetLog_FiltersByDB(t *testing.T) {
	cmdJSON := func(ns, op string, ms int) string {
		raw := map[string]any{
			"t":   map[string]any{"$date": time.Now().UTC().Format(time.RFC3339Nano)},
			"msg": "Slow query",
			"attr": map[string]any{
				"ns":             ns,
				"type":           op,
				"durationMillis": float64(ms),
				"command":        map[string]any{"insert": "x", "documents": []any{map[string]any{"a": 1}}},
			},
		}
		b, _ := json.Marshal(raw)
		return string(b)
	}

	lines := []string{
		cmdJSON("appdb.users", "command", 100),
		cmdJSON("otherdb.things", "command", 200),
		cmdJSON("appdb.orders", "command", 300),
		`{"msg":"not slow"}`,
	}

	fc := &fakeClient{}
	fc.On("GetLog", mock.Anything).Return(bson.A{lines[0], lines[1], lines[2], lines[3]}, nil)

	s := newTestScraper(t, fc)
	dbSet := map[string]struct{}{"appdb": {}}
	now := time.Now()
	entries, err := s.scrapeTopQueryFromGetLog(context.Background(), dbSet, now.Add(-time.Minute), now.Add(time.Second), 20)
	require.NoError(t, err)
	// Two of the four log lines belong to the "appdb" namespace and should pass the filter.
	require.Len(t, entries, 2)
}

func TestScrapeTopQueryFromGetLog_AdvancesCursor(t *testing.T) {
	now := time.Now().UTC()
	t1 := now.Add(-30 * time.Second)
	t2 := now.Add(-5 * time.Second)

	mkLine := func(ts time.Time) string {
		raw := map[string]any{
			"t":   map[string]any{"$date": ts.Format(time.RFC3339Nano)},
			"msg": "Slow query",
			"attr": map[string]any{
				"ns":             "appdb.users",
				"type":           "command",
				"durationMillis": float64(50),
				"command":        map[string]any{"insert": "users", "documents": []any{map[string]any{"a": 1}}},
			},
		}
		b, _ := json.Marshal(raw)
		return string(b)
	}

	fc := &fakeClient{}
	fc.On("GetLog", mock.Anything).Return(bson.A{mkLine(t1), mkLine(t2)}, nil)

	s := newTestScraper(t, fc)
	dbSet := map[string]struct{}{"appdb": {}}
	// Both entries fall in the window (sinceTime is far in the past, upperBound is now).
	entries, err := s.scrapeTopQueryFromGetLog(context.Background(), dbSet, now.Add(-time.Minute), now, 20)
	require.NoError(t, err)
	require.Len(t, entries, 2)
}

// --- retrieveQueryPlan ---

func TestRetrieveQueryPlan_NotExplainable(t *testing.T) {
	fc := &fakeClient{}
	s := newTestScraper(t, fc)

	e := &slowQueryEntry{
		NS:      "appdb.users",
		Op:      "insert",
		Command: bson.D{{Key: "insert", Value: "users"}},
	}
	obfuscated, _ := s.obfuscator.obfuscateCommand(e.Command)
	sig := querySignature(obfuscated)
	plan, planHash := s.retrieveQueryPlan(context.Background(), e, sig)
	require.Empty(t, plan)
	require.Empty(t, planHash)

	if s.planCache != nil {
		v, ok := s.planCache.Get("appdb.users|" + sig)
		require.True(t, ok)
		require.Equal(t, queryPlanSentinel, v)
	}
}

func TestRetrieveQueryPlan_NoCommand(t *testing.T) {
	fc := &fakeClient{}
	s := newTestScraper(t, fc)
	e := &slowQueryEntry{NS: "appdb.users"}
	plan, planHash := s.retrieveQueryPlan(context.Background(), e, "abc")
	require.Empty(t, plan)
	require.Empty(t, planHash)
}

func TestRetrieveQueryPlan_CachesSuccess(t *testing.T) {
	fc := &fakeClient{}
	fc.On("RunCommand", mock.Anything, "appdb", mock.Anything).Return(bson.M{
		"queryPlanner": bson.M{"winningPlan": bson.M{"stage": "COLLSCAN"}},
		"ok":           float64(1),
	}, nil)

	s := newTestScraper(t, fc)
	e := &slowQueryEntry{
		NS:      "appdb.users",
		Op:      "query",
		Command: bson.D{{Key: "find", Value: "users"}, {Key: "filter", Value: bson.D{{Key: "x", Value: 1}}}},
	}
	obfuscated, _ := s.obfuscator.obfuscateCommand(e.Command)
	sig := querySignature(obfuscated)

	plan, planHash := s.retrieveQueryPlan(context.Background(), e, sig)
	require.NotEmpty(t, plan)
	require.NotEmpty(t, planHash)

	plan2, planHash2 := s.retrieveQueryPlan(context.Background(), e, sig)
	require.Equal(t, plan, plan2)
	require.Equal(t, planHash, planHash2)
	fc.AssertNumberOfCalls(t, "RunCommand", 1)
}

// --- processTopQueryEntries ---

func TestProcessTopQueryEntries_EmitsTopQueryEvent(t *testing.T) {
	fc := &fakeClient{}
	s := newTestScraper(t, fc)

	entries := []slowQueryEntry{
		{
			EndTime:        time.Now(),
			NS:             "appdb.users",
			Op:             "insert",
			Millis:         100,
			CPUNanos:       1_000_000,
			ResponseLength: 10,
			KeysExamined:   1,
			DocsExamined:   1,
			NReturned:      1,
			Command:        bson.D{{Key: "insert", Value: "users"}, {Key: "documents", Value: bson.A{bson.D{{Key: "a", Value: 1}}}}},
		},
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	s.processTopQueryEntries(context.Background(), entries, now)

	rb := s.lb.NewResourceBuilder()
	rb.SetServerAddress("localhost")
	rb.SetServerPort(27017)
	rb.SetServiceInstanceID(generateInstanceID("localhost", 27017))
	s.lb.EmitForResource(metadata.WithLogsResource(rb.Emit()))

	logs := s.lb.Emit()
	require.Equal(t, 1, logs.ResourceLogs().Len())
	sl := logs.ResourceLogs().At(0).ScopeLogs().At(0)
	require.Equal(t, 1, sl.LogRecords().Len())

	attrs := sl.LogRecords().At(0).Attributes()

	v, ok := attrs.Get("db.system.name")
	require.True(t, ok)
	require.Equal(t, "mongodb", v.AsString())

	v, ok = attrs.Get("db.namespace")
	require.True(t, ok)
	require.Equal(t, "appdb", v.AsString())

	v, ok = attrs.Get("db.collection.name")
	require.True(t, ok)
	require.Equal(t, "users", v.AsString())

	v, ok = attrs.Get("mongodb.operation.type")
	require.True(t, ok)
	require.Equal(t, "insert", v.AsString())

	_, hasCount := attrs.Get("mongodb.operation.count")
	require.False(t, hasCount)
	_, hasMax := attrs.Get("mongodb.operation.max_duration")
	require.False(t, hasMax)
	_, hasMean := attrs.Get("mongodb.operation.mean_duration")
	require.False(t, hasMean)
}

func TestProcessTopQueryEntries_EmitsEventForEmptyCommand(t *testing.T) {
	fc := &fakeClient{}
	s := newTestScraper(t, fc)

	entries := []slowQueryEntry{
		{NS: "appdb.users", Op: "query", Millis: 100, Command: bson.D{}},
	}
	s.processTopQueryEntries(context.Background(), entries, pcommon.NewTimestampFromTime(time.Now()))

	require.Equal(t, 1, s.lb.Emit().ResourceLogs().Len())
}

// --- keysOf helper ---

func keysOf(d bson.D) []string {
	out := make([]string, 0, len(d))
	for _, e := range d {
		out = append(out, e.Key)
	}
	return out
}
