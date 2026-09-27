// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connectedLines are structurally similar log lines used across several tests.
// The first three tokens ("connected to host") are identical, which routes them
// to the same leaf node in the Drain parse tree for similarity scoring.
var connectedLines = []string{
	"connected to host 10.0.0.1 on port 443",
	"connected to host 192.168.1.1 on port 8080",
	"connected to host 172.16.0.1 on port 80",
}

func defaultCfg() Config {
	return Config{
		Depth:        5,
		SimThreshold: 0.4,
		MaxChildren:  100,
		MaxClusters:  0, // unlimited
	}
}

func TestNewDrain(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)
	require.NotNil(t, d)
}

// TestTrainSimilarLinesShareTemplate verifies that structurally similar log
// lines are merged into the same cluster and produce a template with wildcards.
//
// The first three tokens must be identical for go-drain3's prefix tree to route
// all lines to the same leaf node where similarity scoring can merge them.
func TestTrainSimilarLinesShareTemplate(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	// "connected to host <IP> on port <PORT>" — first 3 tokens identical
	var templates []string
	for _, line := range connectedLines {
		tmpl, _, err := d.Train(line)
		require.NoError(t, err)
		templates = append(templates, tmpl)
	}

	// The first line creates a new cluster with itself as the template; abstraction
	// kicks in once a second similar line is seen. Lines 1 and 2 should share the
	// same abstracted template.
	assert.Equal(t, templates[1], templates[2], "lines should converge on the same template")
	assert.Contains(t, templates[2], "<*>", "merged template should contain wildcard tokens")
}

// TestTrainDistinctLinesGetDifferentClusters confirms that structurally
// unrelated lines produce separate clusters.
func TestTrainDistinctLinesGetDifferentClusters(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	tmpl1, _, err1 := d.Train("connected to host 10.0.0.1 on port 443")
	require.NoError(t, err1)
	tmpl2, _, err2 := d.Train("disk write error on device sda")
	require.NoError(t, err2)

	assert.NotEqual(t, tmpl1, tmpl2, "structurally different lines should get different templates")
}

// TestMatchAfterTemplateAbstracts verifies that Match finds an existing cluster
// once its template has been abstracted (i.e. after multiple similar lines have
// been trained).
func TestMatchAfterTemplateAbstracts(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	// Build the cluster with enough examples to abstract the template.
	var trainTmpl string
	for _, line := range connectedLines {
		var err error
		trainTmpl, _, err = d.Train(line)
		require.NoError(t, err)
	}

	// Match a new, unseen line with the same structure.
	matchTmpl, ok := d.Match("connected to host 10.10.10.10 on port 9000")
	require.True(t, ok, "line matching the abstracted template should be found")
	assert.Equal(t, trainTmpl, matchTmpl)
	assert.Contains(t, matchTmpl, "<*>")
}

// TestMatchDoesNotCreateClusters confirms that Match on an empty tree always
// returns ok=false.
func TestMatchDoesNotCreateClusters(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	_, ok := d.Match("some log line with no prior training")
	assert.False(t, ok)
}

// TestSnapshotRoundtrip verifies that tree state can be serialized and restored.
func TestSnapshotRoundtrip(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	var trainTmpl string
	for _, line := range connectedLines {
		var trainErr error
		trainTmpl, _, trainErr = d.Train(line)
		require.NoError(t, trainErr)
	}

	snap, err := d.Snapshot()
	require.NoError(t, err)
	require.NotEmpty(t, snap)

	d2, err := NewDrain(defaultCfg())
	require.NoError(t, err)
	require.NoError(t, d2.Load(snap))

	matchTmpl, ok := d2.Match("connected to host 10.10.10.10 on port 9000")
	require.True(t, ok, "restored drain should match lines fitting the trained template")
	assert.Equal(t, trainTmpl, matchTmpl)
}

// TestTrainReturnsTokensConsistentWithTemplate verifies that the token slice
// returned by Train, when joined on " ", equals the template string.
func TestTrainReturnsTokensConsistentWithTemplate(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	for _, line := range connectedLines {
		tmpl, tokens, err := d.Train(line)
		require.NoError(t, err)
		require.NotEmpty(t, tokens)
		assert.Equal(t, tmpl, strings.Join(tokens, " "), "joined tokens should equal the returned template")
	}
}

// TestTokeniseMatchesDrainSplit verifies that Tokenise mirrors go-drain3's
// internal getContentAsTokens: trim, replace extra delimiters with spaces,
// then split on " ". Without configuring extra delimiters, no replacement
// happens, so this case validates trim + split.
func TestTokeniseMatchesDrainSplit(t *testing.T) {
	d, err := NewDrain(defaultCfg())
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b", "c"}, d.Tokenise("a b c"))
	assert.Equal(t, []string{"a", "b"}, d.Tokenise("  a b  "))
	assert.Nil(t, d.Tokenise("   "))
	assert.Nil(t, d.Tokenise(""))
}

// TestTokeniseWithExtraDelimiters verifies that extra delimiters are replaced
// with spaces before splitting, matching drain's tokenisation.
func TestTokeniseWithExtraDelimiters(t *testing.T) {
	cfg := defaultCfg()
	cfg.ExtraDelimiters = []string{":"}
	d, err := NewDrain(cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"key", "val", "foo"}, d.Tokenise("key:val foo"))
}

func TestUnlimitedMaxClusters(t *testing.T) {
	cfg := defaultCfg()
	cfg.MaxClusters = 0
	d, err := NewDrain(cfg)
	require.NoError(t, err)
	require.NotNil(t, d)
}
