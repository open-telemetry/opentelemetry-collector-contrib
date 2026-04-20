// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package drain wraps the go-drain3 library behind a minimal, stable API used
// by the drain processor. All callers go through this package; the underlying
// library can be swapped without touching the processor.
//
// Thread safety: Drain is NOT goroutine-safe. Callers must serialize access
// (e.g. with a sync.Mutex).
package drain // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/drain"

import (
	"encoding/json"
	"math"

	drain3 "github.com/jaeyo/go-drain3/pkg/drain3"
)

// Config holds parameters for the Drain parse tree.
type Config struct {
	// Depth is the maximum depth of the parse tree. Minimum 3 (go-drain3 requirement).
	Depth int
	// SimThreshold is the similarity threshold in [0.0, 1.0].
	SimThreshold float64
	// MaxChildren is the maximum number of children per tree node.
	MaxChildren int
	// MaxClusters limits the number of tracked clusters via LRU eviction.
	// 0 means effectively unlimited (internally mapped to math.MaxInt32).
	MaxClusters int
	// ExtraDelimiters are additional token delimiters beyond whitespace.
	ExtraDelimiters []string
}

// Drain wraps the go-drain3 log clustering engine.
type Drain struct {
	inner *drain3.Drain
}

// NewDrain constructs a Drain instance from the provided Config.
func NewDrain(cfg Config) (*Drain, error) {
	maxClusters := cfg.MaxClusters
	if maxClusters <= 0 {
		// go-drain3 uses an LRU which requires a positive size; use MaxInt32 as
		// "effectively unlimited" — the LRU doesn't pre-allocate so this is safe.
		maxClusters = math.MaxInt32
	}

	inner, err := drain3.NewDrain(
		drain3.WithDepth(int64(cfg.Depth)),
		drain3.WithSimTh(cfg.SimThreshold),
		drain3.WithMaxChildren(int64(cfg.MaxChildren)),
		drain3.WithMaxCluster(maxClusters),
		drain3.WithExtraDelimiter(cfg.ExtraDelimiters),
	)
	if err != nil {
		return nil, err
	}
	return &Drain{inner: inner}, nil
}

// Train feeds line to the Drain tree, updating or creating a cluster.
// Returns the derived template string.
// An error is returned only on internal go-drain3 failures; callers should
// log a warning and skip annotation rather than failing the pipeline.
func (d *Drain) Train(line string) (templateStr string, err error) {
	cluster, _, err := d.inner.AddLogMessage(line)
	if err != nil {
		return "", err
	}
	if cluster == nil {
		// go-drain3 returned no cluster without an error; treat as unannotatable.
		return "", nil
	}
	return cluster.GetTemplate(), nil
}

// Match searches the existing tree for a cluster matching line without
// creating new clusters. Returns ok=false if no cluster matches.
func (d *Drain) Match(line string) (templateStr string, ok bool) {
	cluster, err := d.inner.Match(line, drain3.SearchStrategyFallback)
	if err != nil || cluster == nil {
		return "", false
	}
	return cluster.GetTemplate(), true
}

// ClusterCount returns the number of clusters currently tracked in the tree.
// Must be called with the caller's mutex held if concurrent access is possible.
func (d *Drain) ClusterCount() int {
	return len(d.inner.GetClusters())
}

// Snapshot serializes the current tree state to JSON.
func (d *Drain) Snapshot() ([]byte, error) {
	return json.Marshal(d.inner)
}

// Load restores tree state from a previously captured Snapshot.
func (d *Drain) Load(data []byte) error {
	return json.Unmarshal(data, d.inner)
}
