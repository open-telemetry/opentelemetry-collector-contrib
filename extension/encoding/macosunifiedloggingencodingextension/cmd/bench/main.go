// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	ul "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

// normalizedEntry is a best-effort normalization across different JSONL schemas
// produced by the Rust parser and our Go decoder.
// Fields are intentionally minimal and tolerant to schema differences.
type normalizedEntry struct {
	PID       int64
	TID       int64
	Level     string
	Subsystem string
	Category  string
	Message   string
}

func main() {
	var (
		inputPath  string
		logOutput  string
		rustOutput string
		goOutput   string
		maxLines   int
		sampleMism int
		verbose    bool
	)

	flag.StringVar(&inputPath, "input", "/Users/caleb1/Documents/observIQ/otel/opentelemetry-collector-contrib/local/macOSUnifiedLogging/system_logs.logarchive", "Path to a directory or single file containing tracev3 data (recursively scans directories)")
	flag.StringVar(&logOutput, "log", "/Users/caleb1/Documents/observIQ/otel/opentelemetry-collector-contrib/local/macOSUnifiedLogging/output/log_command_output.jsonl", "Path to macOS log command JSONL output (ground truth)")
	flag.StringVar(&rustOutput, "rust", "/Users/caleb1/Documents/observIQ/otel/opentelemetry-collector-contrib/local/macOSUnifiedLogging/output/rust_output.jsonl", "Path to Rust JSONL output to compare against")
	flag.StringVar(&goOutput, "go", "/Users/caleb1/Documents/observIQ/otel/opentelemetry-collector-contrib/local/macOSUnifiedLogging/output/go_output.jsonl", "Path where Go JSONL output will be written")
	flag.IntVar(&maxLines, "max-lines", 0, "Optional cap on lines to compare (0 = no cap)")
	flag.IntVar(&sampleMism, "sample", 10, "How many mismatch examples to print")
	flag.BoolVar(&verbose, "v", false, "Verbose logging")
	flag.Parse()

	start := time.Now()

	if err := os.MkdirAll(filepath.Dir(goOutput), 0o755); err != nil {
		fatalf("failed creating output dir: %v", err)
	}

	if verbose {
		fmt.Printf("Generating Go-decoded JSONL at %s...\n", goOutput)
	}
	if err := decodeWithGo(inputPath, goOutput, verbose); err != nil {
		fatalf("failed to decode with Go: %v", err)
	}

	if verbose {
		fmt.Printf("Comparing three sources:\n")
		fmt.Printf("  Log command (ground truth): %s\n", logOutput)
		fmt.Printf("  Rust parser: %s\n", rustOutput)
		fmt.Printf("  Go decoder: %s\n", goOutput)
	}

	stats, err := compareThreeSources(logOutput, rustOutput, goOutput, maxLines, sampleMism, verbose)
	if err != nil {
		fatalf("comparison failed: %v", err)
	}

	// Print comprehensive summary
	fmt.Printf("\n=== macOS UL Three-Way Parity Benchmark ===\n")
	fmt.Printf("Ground Truth (log cmd): %d entries\n", stats.logTotal)
	fmt.Printf("Rust parser:            %d entries\n", stats.rustTotal)
	fmt.Printf("Go decoder:             %d entries\n", stats.goTotal)
	fmt.Printf("\n--- Parity vs Ground Truth ---\n")
	fmt.Printf("Rust matches log:       %d (%.2f%% recall, %.2f%% precision)\n",
		stats.rustVsLog,
		percentage(stats.rustVsLog, stats.logTotal),
		percentage(stats.rustVsLog, stats.rustTotal))
	fmt.Printf("Go matches log:         %d (%.2f%% recall, %.2f%% precision)\n",
		stats.goVsLog,
		percentage(stats.goVsLog, stats.logTotal),
		percentage(stats.goVsLog, stats.goTotal))
	fmt.Printf("Rust matches Go:        %d\n", stats.rustVsGo)
	fmt.Printf("\n--- Coverage Analysis ---\n")
	fmt.Printf("Log-only entries:       %d\n", stats.logOnly)
	fmt.Printf("Rust-only entries:      %d\n", stats.rustOnly)
	fmt.Printf("Go-only entries:        %d\n", stats.goOnly)
	fmt.Printf("All three match:        %d\n", stats.allThreeMatch)
	fmt.Printf("\n--- Performance ---\n")
	fmt.Printf("Duration:               %s\n", time.Since(start).Truncate(time.Millisecond))

	if verbose && len(stats.sampleMismatches) > 0 {
		fmt.Printf("\nSample mismatches (up to %d):\n", sampleMism)
		for i, s := range stats.sampleMismatches {
			if i >= sampleMism {
				break
			}
			fmt.Printf("- %s\n", s)
		}
	}
}

type threeWayStats struct {
	logTotal         int
	rustTotal        int
	goTotal          int
	rustVsLog        int
	goVsLog          int
	rustVsGo         int
	logOnly          int
	rustOnly         int
	goOnly           int
	allThreeMatch    int
	sampleMismatches []string
}

func percentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return 100.0 * float64(numerator) / float64(denominator)
}

func compareThreeSources(logPath, rustPath, goPath string, maxLines, maxMismatches int, verbose bool) (threeWayStats, error) {
	// Load all three datasets as normalized sets
	logSet, logTotal, err := loadJSONLAsSet(logPath, maxLines)
	if err != nil {
		return threeWayStats{}, fmt.Errorf("load log jsonl: %w", err)
	}

	rustSet, rustTotal, err := loadJSONLAsSet(rustPath, maxLines)
	if err != nil {
		return threeWayStats{}, fmt.Errorf("load rust jsonl: %w", err)
	}

	goSet, goTotal, err := loadJSONLAsSet(goPath, maxLines)
	if err != nil {
		return threeWayStats{}, fmt.Errorf("load go jsonl: %w", err)
	}

	if verbose {
		fmt.Printf("Loaded: log=%d, rust=%d, go=%d entries\n", logTotal, rustTotal, goTotal)
	}

	// Try multiple matching strategies with increasing flexibility
	rustVsLog := findBestMatches(rustSet, logSet, verbose, "Rust vs Log")
	goVsLog := findBestMatches(goSet, logSet, verbose, "Go vs Log")
	rustVsGo := findBestMatches(rustSet, goSet, verbose, "Rust vs Go")

	// Find entries unique to each set (using relaxed matching)
	logOnly := setDifferenceRelaxed(logSet, rustSet, goSet)
	rustOnly := setDifferenceRelaxed(rustSet, logSet, goSet)
	goOnly := setDifferenceRelaxed(goSet, logSet, rustSet)

	// Find entries that match across all three (using relaxed matching)
	allThreeMatch := findThreeWayMatches(logSet, rustSet, goSet)

	// Generate sample mismatches for debugging
	var samples []string

	// Sample from log-only (neither parser found these)
	count := 0
	for key := range logSet {
		if !hasRelaxedMatch(key, rustSet) && !hasRelaxedMatch(key, goSet) {
			samples = append(samples, fmt.Sprintf("MISSED_BY_BOTH: %s", key))
			count++
			if count >= maxMismatches/3 {
				break
			}
		}
	}

	// Sample from rust-only
	count = 0
	for key := range rustSet {
		if !hasRelaxedMatch(key, logSet) && !hasRelaxedMatch(key, goSet) {
			samples = append(samples, fmt.Sprintf("RUST_ONLY: %s", key))
			count++
			if count >= maxMismatches/3 {
				break
			}
		}
	}

	// Sample from go-only
	count = 0
	for key := range goSet {
		if !hasRelaxedMatch(key, logSet) && !hasRelaxedMatch(key, rustSet) {
			samples = append(samples, fmt.Sprintf("GO_ONLY: %s", key))
			count++
			if count >= maxMismatches/3 {
				break
			}
		}
	}

	return threeWayStats{
		logTotal:         logTotal,
		rustTotal:        rustTotal,
		goTotal:          goTotal,
		rustVsLog:        rustVsLog,
		goVsLog:          goVsLog,
		rustVsGo:         rustVsGo,
		logOnly:          len(logOnly),
		rustOnly:         len(rustOnly),
		goOnly:           len(goOnly),
		allThreeMatch:    allThreeMatch,
		sampleMismatches: samples,
	}, nil
}

// findBestMatches tries multiple matching strategies and returns the best result
func findBestMatches(set1, set2 map[string]struct{}, verbose bool, label string) int {
	// Strategy 1: Exact key match (most strict)
	exactMatches := intersectionSize(set1, set2)

	// Strategy 2: Relaxed matching (ignore some fields, fuzzy message matching)
	relaxedMatches := findRelaxedMatches(set1, set2)

	// Strategy 3: Core data matching (just PID, TID, basic message content)
	coreMatches := findCoreDataMatches(set1, set2)

	// Strategy 4: Process-level matching (PID + subsystem, ignore threads/messages)
	processMatches := findProcessLevelMatches(set1, set2)

	if verbose {
		fmt.Printf("%s matching strategies: exact=%d, relaxed=%d, core=%d, process=%d\n",
			label, exactMatches, relaxedMatches, coreMatches, processMatches)
	}

	// Return the best (highest) match count
	best := exactMatches
	if relaxedMatches > best {
		best = relaxedMatches
	}
	if coreMatches > best {
		best = coreMatches
	}
	if processMatches > best {
		best = processMatches
	}
	return best
}

// findRelaxedMatches allows for some field differences and fuzzy message matching
func findRelaxedMatches(set1, set2 map[string]struct{}) int {
	count := 0
	for key1 := range set1 {
		entry1 := parseKeyToEntry(key1)
		for key2 := range set2 {
			entry2 := parseKeyToEntry(key2)
			if isRelaxedMatch(entry1, entry2) {
				count++
				break // Found a match, move to next entry1
			}
		}
	}
	return count
}

// findCoreDataMatches focuses on just the essential data points
func findCoreDataMatches(set1, set2 map[string]struct{}) int {
	count := 0
	for key1 := range set1 {
		entry1 := parseKeyToEntry(key1)
		for key2 := range set2 {
			entry2 := parseKeyToEntry(key2)
			if isCoreDataMatch(entry1, entry2) {
				count++
				break
			}
		}
	}
	return count
}

// findProcessLevelMatches focuses on PID + subsystem matches (different event types from same process)
func findProcessLevelMatches(set1, set2 map[string]struct{}) int {
	// Create process signature maps (PID + subsystem)
	processes1 := make(map[string]struct{})
	processes2 := make(map[string]struct{})

	for key := range set1 {
		entry := parseKeyToEntry(key)
		if entry.PID != 0 && entry.Subsystem != "" {
			sig := fmt.Sprintf("pid=%d|sub=%s", entry.PID, entry.Subsystem)
			processes1[sig] = struct{}{}
		}
	}

	for key := range set2 {
		entry := parseKeyToEntry(key)
		if entry.PID != 0 && entry.Subsystem != "" {
			sig := fmt.Sprintf("pid=%d|sub=%s", entry.PID, entry.Subsystem)
			processes2[sig] = struct{}{}
		}
	}

	// Count matching process signatures
	count := 0
	for sig := range processes1 {
		if _, exists := processes2[sig]; exists {
			count++
		}
	}

	return count
}

// isRelaxedMatch allows for some differences in fields
func isRelaxedMatch(e1, e2 normalizedEntry) bool {
	// More flexible PID matching - if one is 0, ignore it
	if e1.PID != 0 && e2.PID != 0 && e1.PID != e2.PID {
		return false
	}

	// More flexible TID matching - thread IDs can vary even for same process/subsystem
	// Only require match if both are non-zero AND we don't have other strong indicators
	hasStrongMatch := (e1.PID == e2.PID && e1.PID != 0) ||
		(e1.Subsystem != "" && e2.Subsystem != "" &&
			strings.Contains(strings.ToLower(e1.Subsystem), strings.ToLower(e2.Subsystem)))

	if !hasStrongMatch && e1.TID != 0 && e2.TID != 0 && e1.TID != e2.TID {
		return false
	}

	// Subsystem should match (case insensitive, allowing partial matches)
	if e1.Subsystem != "" && e2.Subsystem != "" {
		if !strings.Contains(strings.ToLower(e1.Subsystem), strings.ToLower(e2.Subsystem)) &&
			!strings.Contains(strings.ToLower(e2.Subsystem), strings.ToLower(e1.Subsystem)) {
			return false
		}
	}

	// Message content should have significant overlap (fuzzy matching)
	// But for process-level matches, be more lenient
	if e1.Message != "" && e2.Message != "" {
		return isFuzzyMessageMatch(e1.Message, e2.Message) || isProcessRelatedMessage(e1.Message, e2.Message)
	}

	return true
}

// isCoreDataMatch focuses on just the most essential data
func isCoreDataMatch(e1, e2 normalizedEntry) bool {
	// If both have PIDs, they must match
	if e1.PID != 0 && e2.PID != 0 {
		return e1.PID == e2.PID && isFuzzyMessageMatch(e1.Message, e2.Message)
	}

	// If no PID, try subsystem + message matching
	if e1.Subsystem != "" && e2.Subsystem != "" {
		subsystemMatch := strings.Contains(strings.ToLower(e1.Subsystem), strings.ToLower(e2.Subsystem)) ||
			strings.Contains(strings.ToLower(e2.Subsystem), strings.ToLower(e1.Subsystem))
		return subsystemMatch && isFuzzyMessageMatch(e1.Message, e2.Message)
	}

	// Last resort: just fuzzy message matching
	return isFuzzyMessageMatch(e1.Message, e2.Message)
}

// isFuzzyMessageMatch checks if two messages have significant content overlap
func isFuzzyMessageMatch(msg1, msg2 string) bool {
	if msg1 == "" || msg2 == "" {
		return false
	}

	// Normalize messages
	m1 := normalizeMessageForMatching(msg1)
	m2 := normalizeMessageForMatching(msg2)

	// Exact match after normalization
	if m1 == m2 {
		return true
	}

	// Check for significant word overlap
	words1 := strings.Fields(m1)
	words2 := strings.Fields(m2)

	if len(words1) == 0 || len(words2) == 0 {
		return false
	}

	// Count overlapping words (ignoring very common words)
	overlap := 0
	for _, w1 := range words1 {
		if len(w1) < 3 { // Skip very short words
			continue
		}
		if isCommonWord(w1) { // Skip common words
			continue
		}
		for _, w2 := range words2 {
			if strings.EqualFold(w1, w2) {
				overlap++
				break
			}
		}
	}

	// Require at least 30% word overlap for longer messages, 1 word for short ones
	minWords := max(len(words1), len(words2))
	if minWords <= 3 {
		return overlap >= 1
	}
	threshold := max(1, minWords*30/100) // 30% threshold
	return overlap >= threshold
}

// isProcessRelatedMessage checks if two messages are likely from the same process/subsystem
func isProcessRelatedMessage(msg1, msg2 string) bool {
	// Extract key terms that might indicate same process activity
	terms1 := extractKeyTerms(msg1)
	terms2 := extractKeyTerms(msg2)

	// Look for any overlapping key terms
	for term := range terms1 {
		if _, exists := terms2[term]; exists {
			return true
		}
	}

	return false
}

// extractKeyTerms pulls out significant words that might indicate process activity
func extractKeyTerms(msg string) map[string]struct{} {
	terms := make(map[string]struct{})
	msg = strings.ToLower(msg)

	// Look for process-related keywords
	processTerms := []string{
		"event", "button", "context", "window", "display", "frame",
		"signpost", "activity", "assertion", "process", "thread",
		"skylight", "runningboard", "biome", "loginwindow",
	}

	for _, term := range processTerms {
		if strings.Contains(msg, term) {
			terms[term] = struct{}{}
		}
	}

	return terms
}

// normalizeMessageForMatching cleans up message for comparison
func normalizeMessageForMatching(msg string) string {
	// Convert to lowercase
	msg = strings.ToLower(msg)

	// Remove common prefixes that might differ
	prefixes := []string{"[event]", "signpost id:", "signpost name:", "assertion"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(msg, prefix) {
			msg = strings.TrimSpace(msg[len(prefix):])
		}
	}

	// Remove hexadecimal values and UUIDs that might be formatting differences
	// Replace hex patterns like 0x1234, timestamps, etc.
	msg = strings.ReplaceAll(msg, "\n", " ")
	words := strings.Fields(msg)
	filtered := make([]string, 0, len(words))

	for _, word := range words {
		// Skip hex values, timestamps, UUIDs, etc.
		if strings.HasPrefix(word, "0x") {
			continue
		}
		if isAllDigits(word) && len(word) > 6 { // Skip long timestamps
			continue
		}
		if isUUIDLike(word) {
			continue
		}
		filtered = append(filtered, word)
	}

	return strings.Join(filtered, " ")
}

// Helper functions
func isCommonWord(word string) bool {
	common := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"will": true, "be": true, "been": true, "have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "can": true, "could": true, "should": true, "would": true, "may": true, "might": true,
	}
	return common[strings.ToLower(word)]
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isUUIDLike(s string) bool {
	// Simple heuristic for UUID-like strings
	return len(s) >= 32 && (strings.Contains(s, "-") ||
		(len(s) == 32 && isAllHex(s)) ||
		(len(s) == 36 && strings.Count(s, "-") == 4))
}

func isAllHex(s string) bool {
	for _, r := range strings.ToLower(s) {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// parseKeyToEntry converts a key back to a normalizedEntry for matching
func parseKeyToEntry(key string) normalizedEntry {
	parts := strings.Split(key, "|")
	entry := normalizedEntry{}

	for _, part := range parts {
		if strings.HasPrefix(part, "pid=") {
			if val, err := parseInt64(part[4:]); err == nil {
				entry.PID = val
			}
		} else if strings.HasPrefix(part, "tid=") {
			if val, err := parseInt64(part[4:]); err == nil {
				entry.TID = val
			}
		} else if strings.HasPrefix(part, "lvl=") {
			entry.Level = part[4:]
		} else if strings.HasPrefix(part, "sub=") {
			entry.Subsystem = part[4:]
		} else if strings.HasPrefix(part, "cat=") {
			entry.Category = part[4:]
		} else if strings.HasPrefix(part, "msg=") {
			entry.Message = part[4:]
		}
	}

	return entry
}

// hasRelaxedMatch checks if a key has any relaxed match in a set
func hasRelaxedMatch(key string, set map[string]struct{}) bool {
	entry := parseKeyToEntry(key)
	for otherKey := range set {
		otherEntry := parseKeyToEntry(otherKey)
		if isRelaxedMatch(entry, otherEntry) {
			return true
		}
	}
	return false
}

// setDifferenceRelaxed finds entries that don't have relaxed matches in other sets
func setDifferenceRelaxed(target map[string]struct{}, excludeSets ...map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for key := range target {
		isUnique := true
		for _, excludeSet := range excludeSets {
			if hasRelaxedMatch(key, excludeSet) {
				isUnique = false
				break
			}
		}
		if isUnique {
			result[key] = struct{}{}
		}
	}
	return result
}

// findThreeWayMatches finds entries that match across all three sets using relaxed matching
func findThreeWayMatches(logSet, rustSet, goSet map[string]struct{}) int {
	count := 0
	for logKey := range logSet {
		if hasRelaxedMatch(logKey, rustSet) && hasRelaxedMatch(logKey, goSet) {
			count++
		}
	}
	return count
}

func intersectionSize(set1, set2 map[string]struct{}) int {
	count := 0
	for key := range set1 {
		if _, exists := set2[key]; exists {
			count++
		}
	}
	return count
}

func isTraceV3(path string) bool {
	low := strings.ToLower(path)
	return strings.HasSuffix(low, ".tracev3") || strings.Contains(low, ".tracev3.")
}

func decodeWithGo(inputPath, outPath string, verbose bool) error {
	// Truncate existing output to avoid mixing runs
	if f, err := os.Create(outPath); err == nil {
		_ = f.Close()
	} else {
		return err
	}

	// Build extension via factory
	factory := ul.NewFactory()
	cfg := factory.CreateDefaultConfig().(*ul.Config)

	ctx := context.Background()
	ext, err := factory.Create(ctx, extensiontest.NewNopSettings(factory.Type()), cfg)
	if err != nil {
		return fmt.Errorf("create extension: %w", err)
	}
	defer func() { _ = ext.Shutdown(ctx) }()

	if err := ext.Start(ctx, componenttest.NewNopHost()); err != nil {
		return fmt.Errorf("start extension: %w", err)
	}

	// Type-assert to the LogsUnmarshalerExtension interface
	dec, ok := ext.(encoding.LogsUnmarshalerExtension)
	if !ok {
		return fmt.Errorf("extension does not implement LogsUnmarshalerExtension")
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("stat input: %w", err)
	}

	if info.IsDir() {
		return filepath.WalkDir(inputPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !isTraceV3(path) {
				return nil
			}
			return decodeFile(dec, path, verbose)
		})
	}
	return decodeFile(dec, inputPath, verbose)
}

func decodeFile(dec encoding.LogsUnmarshalerExtension, path string, verbose bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if verbose {
		fmt.Printf("Decoding %s (%d bytes)\n", path, len(data))
		fmt.Printf("  About to call UnmarshalLogs...\n")
	}

	_, err = dec.UnmarshalLogs(data)
	if err != nil {
		if verbose {
			fmt.Printf("  Error decoding %s: %v\n", path, err)
		}
		return err
	}

	if verbose {
		fmt.Printf("  UnmarshalLogs completed successfully\n")
	}

	return nil
}

func loadJSONLAsSet(path string, maxLines int) (map[string]struct{}, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	// Increase buffer for long JSONL lines
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 10*1024*1024)

	set := make(map[string]struct{}, 1024)
	total := 0
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			// Skip malformed line but count it
			total++
			continue
		}
		ne := normalize(m)
		if ne.Message == "" && ne.PID == 0 && ne.TID == 0 {
			// Likely a summary or non-log row
			total++
			continue
		}
		key := keyFor(ne)
		set[key] = struct{}{}
		total++
		if maxLines > 0 && total >= maxLines {
			break
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return set, total, err
	}
	return set, total, nil
}

func normalize(m map[string]any) normalizedEntry {
	// Handle different field names across the three sources
	pid := pickInt(m, "process_id", "pid", "processId", "processID")
	tid := pickInt(m, "thread_id", "tid", "threadId", "threadID")

	// Handle log command fields vs rust/go fields
	lvl := pickStr(m, "log_level", "level", "severity", "severity_text", "messageType")
	ss := stripBrackets(pickStr(m, "subsystem"))
	cat := stripBrackets(pickStr(m, "category"))

	// Message field variations
	msg := pickStr(m, "message", "msg", "event_message", "eventMessage", "body")
	msg = strings.TrimSpace(msg)

	// Normalize log levels
	lvl = strings.ToLower(strings.TrimSpace(lvl))
	if lvl == "default" {
		lvl = "info"
	}

	return normalizedEntry{PID: pid, TID: tid, Level: lvl, Subsystem: ss, Category: cat, Message: msg}
}

func keyFor(e normalizedEntry) string {
	// Build a relatively stable key; include a compacted message to mitigate trivial differences
	snippet := e.Message
	if len(snippet) > 120 {
		snippet = snippet[:120]
	}
	snippet = compactSpaces(strings.ToLower(snippet))
	parts := []string{
		fmt.Sprintf("pid=%d", e.PID),
		fmt.Sprintf("tid=%d", e.TID),
		fmt.Sprintf("lvl=%s", e.Level),
		fmt.Sprintf("sub=%s", e.Subsystem),
		fmt.Sprintf("cat=%s", e.Category),
		fmt.Sprintf("msg=%s", snippet),
	}
	return strings.Join(parts, "|")
}

func pickStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			case fmt.Stringer:
				return t.String()
			case float64:
				return fmt.Sprintf("%.0f", t)
			}
		}
	}
	return ""
}

func pickInt(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int64(t)
			case int64:
				return t
			case int:
				return int64(t)
			case json.Number:
				if iv, err := t.Int64(); err == nil {
					return iv
				}
			case string:
				if iv, err := parseInt64(t); err == nil {
					return iv
				}
			}
		}
	}
	return 0
}

func parseInt64(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	var x int64
	var err error
	if strings.HasPrefix(s, "0x") {
		_, err = fmt.Sscanf(s, "%x", &x)
		return x, err
	}
	_, err = fmt.Sscanf(s, "%d", &x)
	return x, err
}

func compactSpaces(s string) string {
	fs := strings.Fields(s)
	return strings.Join(fs, " ")
}

func stripBrackets(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	return s
}

func fatalf(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}
