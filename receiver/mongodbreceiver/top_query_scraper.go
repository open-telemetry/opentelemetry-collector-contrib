// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

// slowQueryEntry is the single shared representation of one slow query
// execution
type slowQueryEntry struct {
	TS                 time.Time `bson:"ts"`
	NS                 string    `bson:"ns"`
	Op                 string    `bson:"op"`
	Millis             int64     `bson:"millis"`
	CPUNanos           int64     `bson:"cpuNanos"`
	ResponseLength     int64     `bson:"responseLength"`
	KeysExamined       int64     `bson:"keysExamined"`
	DocsExamined       int64     `bson:"docsExamined"`
	NReturned          int64     `bson:"nreturned"`
	PlanSummary        string    `bson:"planSummary"`
	Command            bson.D    `bson:"command"`
	CursorID           int64     `bson:"cursorid"`
	OriginatingCommand bson.D    `bson:"originatingCommand"`
}

// structuredLogLine is the JSON shape of a MongoDB structured log entry.
type structuredLogLine struct {
	T    json.RawMessage `json:"t"`
	Msg  string          `json:"msg"`
	Attr struct {
		NS                 string          `json:"ns"`
		Type               string          `json:"type"`
		DurationMillis     int64           `json:"durationMillis"`
		CPUNanos           int64           `json:"cpuNanos"`
		Reslen             int64           `json:"reslen"`
		KeysExamined       int64           `json:"keysExamined"`
		DocsExamined       int64           `json:"docsExamined"`
		NReturned          int64           `json:"nreturned"`
		PlanSummary        string          `json:"planSummary"`
		Command            json.RawMessage `json:"command"`
		Cursorid           int64           `json:"cursorid"`
		OriginatingCommand json.RawMessage `json:"originatingCommand"`
	} `json:"attr"`
}

// topN sorts entries by Millis descending and returns the top n.
func topN(entries []slowQueryEntry, n int) []slowQueryEntry {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Millis > entries[j].Millis
	})
	if n == 0 || len(entries) == 0 {
		return nil
	}
	if len(entries) > n {
		return entries[:n]
	}
	return entries
}

const queryPlanSentinel = "__unexplainable__"

var systemDatabases = map[string]struct{}{
	"admin":  {},
	"local":  {},
	"config": {},
}

func buildPlanCache(config *Config, logger *zap.Logger) *lru.LRU[string, string] {
	size := config.TopQueryCollection.QueryPlanCacheSize
	ttl := config.TopQueryCollection.QueryPlanCacheTTL
	if size <= 0 {
		logger.Debug("Query plan caching disabled, not building plan cache",
			zap.Int("query_plan_cache_size", size))
		return nil
	}
	logger.Debug("Building query plan cache",
		zap.Int("query_plan_cache_size", size),
		zap.Duration("query_plan_cache_ttl", ttl))
	return lru.NewLRU[string, string](size, nil, ttl)
}

func (s *mongodbScraper) scrapeTopQueryLogs(ctx context.Context) (plog.Logs, error) {
	if interval := s.config.TopQueryCollection.CollectionInterval; interval > 0 {
		if !s.lastTopQueryExecution.IsZero() && time.Since(s.lastTopQueryExecution) < interval {
			s.logger.Debug("Skipping top_query collection, interval has not elapsed")
			return plog.NewLogs(), nil
		}
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		s.logger.Debug("Failed to get server status for top_query logs", zap.Error(err))
		return plog.NewLogs(), fmt.Errorf("failed to get server status for top_query logs: %w", err)
	}

	serverAddress, serverPort, err := serverAddressAndPort(serverStatus)
	if err != nil {
		s.logger.Debug("Failed to extract server address and port for top_query logs", zap.Error(err))
		return plog.NewLogs(), fmt.Errorf("failed to extract server address and port for top_query logs: %w", err)
	}

	// Use the server's clock as the reference time to avoid collector/server clock skew.
	serverNow := time.Now().UTC()
	if t, ok := serverStatus["localTime"].(bson.DateTime); ok {
		serverNow = t.Time().UTC()
	}

	dbNames, err := s.client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return plog.NewLogs(), fmt.Errorf("failed to list databases: %w", err)
	}

	// Determine the lower bound of the scrape window.
	sinceTime := s.lastScrapeTime
	if sinceTime.IsZero() {
		sinceTime = serverNow.Add(-s.config.TopQueryCollection.CollectionInterval)
	}

	maxRows := s.config.TopQueryCollection.MaxQuerySampleCount
	explainBudget := s.config.TopQueryCollection.MaxExplainEachInterval

	s.logger.Debug("Collecting top query data with config",
		zap.Int64("max_query_sample_count", maxRows),
		zap.Int64("max_explain_each_interval", explainBudget),
		zap.Int64("top_query_count", s.config.TopQueryCollection.TopQueryCount),
		zap.Duration("collection_interval", s.config.TopQueryCollection.CollectionInterval),
	)

	var allEntries []slowQueryEntry
	logFallbackDBs := make(map[string]struct{})

	for _, dbName := range dbNames {
		if _, skip := systemDatabases[dbName]; skip {
			continue
		}
		entries, used, err := s.scrapeTopQueryFromProfiler(ctx, dbName, sinceTime, serverNow, maxRows)
		if err != nil {
			s.logger.Warn("profiler scrape failed, will attempt getLog fallback",
				zap.String("db", dbName), zap.Error(err))
		}
		allEntries = append(allEntries, entries...)
		if !used {
			logFallbackDBs[dbName] = struct{}{}
		}
	}

	if len(logFallbackDBs) > 0 {
		entries, err := s.scrapeTopQueryFromGetLog(ctx, logFallbackDBs, sinceTime, serverNow, maxRows)
		if err != nil {
			s.logger.Warn("getLog fallback failed", zap.Error(err))
		}
		allEntries = append(allEntries, entries...)
	}

	if len(allEntries) > 0 {
		s.processTopQueryEntries(ctx, allEntries, now, explainBudget)
	}

	rb := s.lb.NewResourceBuilder()
	setResourceAttributes(rb, serverAddress, serverPort)
	s.lb.EmitForResource(metadata.WithLogsResource(rb.Emit()))

	s.lastScrapeTime = serverNow
	s.lastTopQueryExecution = time.Now()

	return s.lb.Emit(), nil
}

// scrapeTopQueryFromProfiler fetches up to maxRows slowest ops from `system.profile`
// for a database whose ts falls in (sinceTime, upperBound].
func (s *mongodbScraper) scrapeTopQueryFromProfiler(ctx context.Context, dbName string, sinceTime, upperBound time.Time, maxRows int64) ([]slowQueryEntry, bool, error) {
	level, err := s.getProfilingLevel(ctx, dbName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get profiling level: %w", err)
	}
	if level == 0 {
		s.logger.Debug("profiler disabled, will use getLog fallback", zap.String("db", dbName))
		return nil, false, nil
	}

	docs, err := s.client.FindProfileDocs(ctx, dbName, sinceTime, upperBound, maxRows)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch profile docs: %w", err)
	}

	entries := make([]slowQueryEntry, 0, len(docs))
	for i := range docs {
		if len(docs[i].Command) == 0 {
			continue
		}
		entries = append(entries, docs[i])
	}
	return entries, true, nil
}

func (s *mongodbScraper) getProfilingLevel(ctx context.Context, dbName string) (int, error) {
	result, err := s.client.RunCommand(ctx, dbName, bson.M{"profile": -1})
	if err != nil {
		return 0, err
	}
	was, ok := result["was"]
	if !ok {
		return 0, nil
	}
	switch v := was.(type) {
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	}
	return 0, nil
}

// scrapeTopQueryFromGetLog collects slow query entries from the getLog ring
// buffer whose TS falls in (sinceTime, upperBound], then ranks by millis
// descending on the client side and returns at most topN entries.
func (s *mongodbScraper) scrapeTopQueryFromGetLog(ctx context.Context, dbSet map[string]struct{}, sinceTime, upperBound time.Time, limit int64) ([]slowQueryEntry, error) {
	arr, err := s.client.GetLog(ctx)
	if err != nil {
		return nil, fmt.Errorf("getLog failed: %w", err)
	}

	var entries []slowQueryEntry

	// Iterate in reverse: newest entries first.
	for _, raw := range slices.Backward(arr) {
		line, ok := raw.(string)
		if !ok {
			continue
		}
		entry, ok := parseLogLine(line)
		if !ok {
			continue
		}
		if !entry.TS.After(sinceTime) {
			break
		}
		if entry.TS.After(upperBound) {
			// Written after the scrape's snapshot time; belongs to the next window.
			continue
		}
		if _, want := dbSet[getDBFromNamespace(entry.NS)]; !want {
			continue
		}
		entries = append(entries, entry)
	}

	// Rank by millis desc, keep at most limit.
	return topN(entries, int(limit)), nil
}

func parseLogLine(line string) (slowQueryEntry, bool) {
	var raw structuredLogLine
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return slowQueryEntry{}, false
	}
	if !strings.EqualFold(raw.Msg, "slow query") {
		return slowQueryEntry{}, false
	}
	if len(raw.Attr.Command) == 0 {
		return slowQueryEntry{}, false
	}

	// Decode "t": MongoDB encodes it as {"$date":"2026-01-02T03:04:05Z"} (relaxed Extended JSON).
	var tDoc bson.M
	if err := bson.UnmarshalExtJSON(raw.T, false, &tDoc); err != nil {
		return slowQueryEntry{}, false
	}
	dateStr, ok := tDoc["$date"].(string)
	if !ok {
		return slowQueryEntry{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, dateStr)
	if err != nil {
		return slowQueryEntry{}, false
	}

	var cmd bson.D
	if err := bson.UnmarshalExtJSON(raw.Attr.Command, false, &cmd); err != nil || len(cmd) == 0 {
		return slowQueryEntry{}, false
	}

	var originating bson.D
	if len(raw.Attr.OriginatingCommand) > 0 {
		if err := bson.UnmarshalExtJSON(raw.Attr.OriginatingCommand, false, &originating); err != nil {
			originating = nil
		}
	}

	return slowQueryEntry{
		TS:                 t,
		NS:                 raw.Attr.NS,
		Op:                 raw.Attr.Type,
		Millis:             raw.Attr.DurationMillis,
		CPUNanos:           raw.Attr.CPUNanos,
		ResponseLength:     raw.Attr.Reslen,
		KeysExamined:       raw.Attr.KeysExamined,
		DocsExamined:       raw.Attr.DocsExamined,
		NReturned:          raw.Attr.NReturned,
		PlanSummary:        raw.Attr.PlanSummary,
		Command:            cmd,
		CursorID:           raw.Attr.Cursorid,
		OriginatingCommand: originating,
	}, true
}

func (s *mongodbScraper) processTopQueryEntries(ctx context.Context, entries []slowQueryEntry, now pcommon.Timestamp, explainBudget int64) {
	ranked := topN(entries, int(s.config.TopQueryCollection.TopQueryCount))

	const msPerSec = 1000.0
	for i := range ranked {
		e := &ranked[i]

		queryTruncated, commandComment, operationName := extractCommandMetadata(e.Command, e.Op)

		obfuscated, err := s.obfuscator.obfuscateCommand(cleanCommand(e.Command))
		if err != nil {
			s.logger.Debug("failed to obfuscate command", zap.Error(err))
		}
		sig := querySignature(obfuscated)

		queryPlan, queryPlanHash := s.retrieveQueryPlan(ctx, e, sig, &explainBudget)

		cursorID := fmt.Sprintf("%v", e.CursorID)
		originatingCommand := ""
		if len(e.OriginatingCommand) > 0 {
			obf, obfErr := s.obfuscator.obfuscateCommand(cleanCommand(e.OriginatingCommand))
			if obfErr != nil {
				s.logger.Debug("failed to obfuscate originating command", zap.Error(obfErr))
			}
			originatingCommand = obf
		}

		s.lb.RecordDbServerTopQueryEvent(
			ctx,
			now,
			getCollectionFromNamespace(e.NS),
			getDBFromNamespace(e.NS),
			operationName,
			obfuscated,
			metadata.AttributeDbSystemNameMongodb,
			cursorID,
			originatingCommand,
			queryPlanHash,
			queryPlan,
			commandComment,
			float64(e.CPUNanos)/float64(time.Second),
			e.DocsExamined,
			e.NReturned,
			float64(e.Millis)/msPerSec,
			e.KeysExamined,
			e.PlanSummary,
			e.ResponseLength,
			e.Op,
			queryTruncated,
		)
	}
}

// retrieveQueryPlan fetches (or returns cached) explain plan for the given entry.
// remainingExplainBudget is a per-scrape counter of remaining server-side explain calls;
func (s *mongodbScraper) retrieveQueryPlan(ctx context.Context, e *slowQueryEntry, sig string, remainingExplainBudget *int64) (string, string) {
	if len(e.Command) == 0 {
		return "", ""
	}

	entryDB := getDBFromNamespace(e.NS)
	if entryDB == "" {
		return "", ""
	}

	cacheKey := e.NS + "|" + sig

	if s.planCache != nil {
		if cached, ok := s.planCache.Get(cacheKey); ok {
			if cached == queryPlanSentinel {
				return "", ""
			}
			plan, hash, _ := strings.Cut(cached, "\x00")
			return plan, hash
		}
	}

	cmd := stripKeys(e.Command, commandKeysToStrip)

	if !isExplainable(e.Op, cmd) {
		if s.planCache != nil {
			s.planCache.Add(cacheKey, queryPlanSentinel)
		}
		return "", ""
	}

	if *remainingExplainBudget <= 0 {
		s.logger.Debug("explain budget exhausted for this scrape, skipping explain",
			zap.String("db", entryDB))
		return "", ""
	}
	*remainingExplainBudget--

	prepared := prepareForExplainCleaned(cmd)
	result, err := s.client.RunCommand(ctx, entryDB, bson.D{
		{Key: "explain", Value: prepared},
		{Key: "verbosity", Value: "queryPlanner"},
	})
	if err != nil {
		s.logger.Warn("Failed to run explain",
			zap.String("db", entryDB),
			zap.Error(err),
		)
		if s.planCache != nil {
			s.planCache.Add(cacheKey, queryPlanSentinel)
		}
		return "", ""
	}

	cleaned := cleanExplainResult(result)
	obfuscatedPlan := obfuscateExplainPlan(cleaned)
	plan := marshalJSON(obfuscatedPlan)
	planHash := querySignature(plan)

	if s.planCache != nil {
		s.planCache.Add(cacheKey, plan+"\x00"+planHash)
	}
	return plan, planHash
}
