// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package finder // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"

import "github.com/bmatcuk/doublestar/v4"

func getDefaultDoublestarOptions() []doublestar.GlobOption {
	return []doublestar.GlobOption{}
}

func pathExcluded(excludes []string, path string) bool {
	for _, exclude := range excludes {
		if itMatches, _ := doublestar.PathMatch(exclude, path); itMatches {
			return true
		}
	}
	return false
}
