// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinality

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"testing"
)

func set(ss ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func TestNilControl_NoChange(t *testing.T) {
	lbls := map[string]string{"a": "1", "b": "2"}
	var c *Control

	out := c.Enforce(lbls)
	if !reflect.DeepEqual(out, lbls) {
		t.Fatalf("expected labels unchanged when Control=nil, got: %#v", out)
	}
}

func TestBlocklist_RemovesBlockedKeys(t *testing.T) {
	c := &Control{
		Block: set("debug", "temp"),
	}
	lbls := map[string]string{
		"debug": "x",
		"temp":  "y",
		"keep":  "z",
	}

	got := c.Enforce(lbls)

	if _, ok := got["debug"]; ok {
		t.Fatalf("expected 'debug' to be removed")
	}
	if _, ok := got["temp"]; ok {
		t.Fatalf("expected 'temp' to be removed")
	}
	if v := got["keep"]; v != "z" {
		t.Fatalf("expected 'keep' to remain = z, got %q", v)
	}
}

func TestMaxValLen_TruncatesValue(t *testing.T) {
	c := &Control{
		MaxValLen:     5,
		HashIfExceeds: 0,
	}
	lbls := map[string]string{"k": "abcdefgh"}

	got := c.Enforce(lbls)

	if v := got["k"]; v != "abcde" {
		t.Fatalf("expected truncation to 'abcde', got %q", v)
	}
}

func TestMaxValLen_HashIfExceeds(t *testing.T) {
	c := &Control{
		MaxValLen:     5,
		HashIfExceeds: 1, // enable hashing path
	}
	orig := "abcdefgh"
	lbls := map[string]string{"k": orig}

	got := c.Enforce(lbls)

	wantHash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])[:8]
	}(orig)

	if v := got["k"]; v != wantHash {
		t.Fatalf("expected hash prefix %q, got %q", wantHash, v)
	}
}

func TestMaxTotalSize_DropsNonAllowedFirst_InAlphabeticalOrder(t *testing.T) {
	// Initial sizes (key+value):
	// service: 7 + 3 = 10  (allowed)
	// a:       1 + 2 = 3
	// b:       1 + 2 = 3
	// c:       1 + 2 = 3
	// total = 19. Cap at 15 → delete 'a' (->16) then 'b' (->13). 'c' remains.
	lbls := map[string]string{
		"service": "svc",
		"a":       "xx",
		"b":       "yy",
		"c":       "zz",
	}
	c := &Control{
		MaxTotalSize: 15,
		Allow:        set("service"),
	}

	got := c.Enforce(lbls)

	if _, ok := got["service"]; !ok {
		t.Fatalf("allowed key 'service' must be preserved")
	}
	if _, ok := got["a"]; ok {
		t.Fatalf("'a' should have been dropped first")
	}
	if _, ok := got["b"]; ok {
		t.Fatalf("'b' should have been dropped second to meet cap")
	}
	if _, ok := got["c"]; !ok {
		t.Fatalf("'c' should remain after size trimming")
	}

	// sanity: recompute total
	total := 0
	for k, v := range got {
		total += len(k) + len(v)
	}
	if total > c.MaxTotalSize {
		t.Fatalf("total size %d exceeds cap %d after trimming", total, c.MaxTotalSize)
	}
}

func TestMaxLabels_DropsNonAllowed_DescendingLexicographic(t *testing.T) {
	// Only non-allowed keys get dropped; they’re sorted ascending and removed from the end (i.e., descending).
	// With MaxLabels=2 and two allowed keys, all non-allowed should be removed in order: c, b, a.
	lbls := map[string]string{
		"keep1": "x",
		"keep2": "y",
		"a":     "1",
		"b":     "2",
		"c":     "3",
	}
	c := &Control{
		MaxLabels: 2,
		Allow:     set("keep1", "keep2"),
	}

	got := c.Enforce(lbls)

	if len(got) != 2 {
		t.Fatalf("expected exactly 2 labels remaining, got %d: %#v", len(got), got)
	}
	if _, ok := got["keep1"]; !ok {
		t.Fatalf("'keep1' must remain")
	}
	if _, ok := got["keep2"]; !ok {
		t.Fatalf("'keep2' must remain")
	}
	if _, ok := got["a"]; ok || func() bool { _, ok := got["b"]; return ok }() || func() bool { _, ok := got["c"]; return ok }() {
		t.Fatalf("non-allowed keys should have been dropped to meet MaxLabels")
	}
}

func TestOrderOfOperations_ValueChangesAffectSizeThenLabelCaps(t *testing.T) {
	long := make([]byte, 100)
	for i := range long {
		long[i] = 'x'
	}

	lbls := map[string]string{
		"allowA": "A",
		"allowB": "B",
		"noisy":  string(long), // will be trimmed to 5 chars
	}
	c := &Control{
		MaxValLen:     5,
		HashIfExceeds: 0,  // truncate to 5
		MaxTotalSize:  24, // after truncation total is 24 → no trimming
		MaxLabels:     3,  // no label cap needed
		Allow:         set("allowA", "allowB"),
	}

	got := c.Enforce(lbls)

	// Verify truncation happened before size/label checks
	if v := got["noisy"]; v != "xxxxx" {
		t.Fatalf("expected 'noisy' value truncated to 'xxxxx', got %q", v)
	}

	// Verify we stayed under the total size cap post-truncation
	total := 0
	for k, v := range got {
		total += len(k) + len(v)
	}
	if total > c.MaxTotalSize {
		t.Fatalf("total size %d exceeds cap %d after truncation", total, c.MaxTotalSize)
	}

	// Verify no labels were dropped
	want := []string{"allowA", "allowB", "noisy"}
	var have []string
	for k := range got {
		have = append(have, k)
	}
	sort.Strings(have)
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("expected keys %v, got %v", want, have)
	}
}

func TestCapsCannotRemoveAllowedBeyondLimits(t *testing.T) {
	// If allowed keys alone exceed MaxLabels, function will not remove allowed keys.
	// (Per current logic: only non-allowed are considered for removal.)
	lbls := map[string]string{
		"keep1": "x",
		"keep2": "y",
		"keep3": "z",
	}
	c := &Control{
		MaxLabels: 2,
		Allow:     set("keep1", "keep2", "keep3"),
	}

	got := c.Enforce(lbls)

	// Should still contain all three allowed keys (cannot drop them)
	if len(got) != 3 {
		t.Fatalf("allowed keys should not be removed even if exceeding MaxLabels; got=%#v", got)
	}
}
