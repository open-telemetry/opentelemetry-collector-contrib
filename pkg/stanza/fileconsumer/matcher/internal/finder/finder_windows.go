// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package finder // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/metadata"
)

func getDefaultDoublestarOptions() []doublestar.GlobOption {
	// On Windows, filepaths are case-insensitive by default. As a result,
	// we want our globs to be case-insensitive.

	// This is currently guarded by a featuregate, which will eventually become
	// the default.
	options := []doublestar.GlobOption{}
	if metadata.FilelogWindowsCaseInsensitiveFeatureGate.IsEnabled() {
		options = append(options, doublestar.WithCaseInsensitive())
	}
	return options
}

func pathExcluded(excludes []string, path string) bool {
	// To allow case-insensitive matching, the path and exclude
	// are unified to lowercase before matching.

	if metadata.FilelogWindowsCaseInsensitiveFeatureGate.IsEnabled() {
		lowerPath := strings.ToLower(path)
		for _, exclude := range excludes {
			lowerExclude := strings.ToLower(exclude)
			if itMatches, _ := doublestar.PathMatch(lowerExclude, lowerPath); itMatches {
				return true
			}
		}
		return false
	}

	for _, exclude := range excludes {
		if itMatches, _ := doublestar.PathMatch(exclude, path); itMatches {
			return true
		}
	}
	return false
}

// fixUNCPath corrects UNC path corruption that occurs when doublestar's path.Join
// collapses // to /. If the pattern starts with \\ (UNC) but the match only has \,
// we restore the UNC prefix.
func fixUNCPath(pattern, match string) string {
	// Check if pattern is a UNC path (starts with \\)
	if len(pattern) >= 2 && pattern[0] == '\\' && pattern[1] == '\\' {
		// Check if match is corrupted (starts with single \ instead of \\)
		if len(match) >= 1 && match[0] == '\\' && (len(match) == 1 || match[1] != '\\') {
			// Restore the missing backslash
			return `\` + match
		}
	}
	return match
}
