// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package finder // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

func getDefaultDoublestarOptions() []doublestar.GlobOption {
	// On Windows, filepaths are case-insensitive by default. As a result,
	// we want our globs to be case-insensitive.
	return []doublestar.GlobOption{doublestar.WithCaseInsensitive()}
}

func pathExcluded(excludes []string, path string) bool {
	// To allow case-insensitive matching, the path and exclude
	// are unified to lowercase before matching.
	lowerPath := strings.ToLower(path)
	for _, exclude := range excludes {
		lowerExclude := strings.ToLower(exclude)
		if itMatches, _ := doublestar.PathMatch(lowerExclude, lowerPath); itMatches {
			return true
		}
	}
	return false
}
