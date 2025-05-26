// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package finder // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"

import (
	"fmt"
	"maps"
	"slices"

	"github.com/bmatcuk/doublestar/v4"
	"go.uber.org/multierr"
)

func Validate(globs []string) error {
	for _, glob := range globs {
		_, err := doublestar.PathMatch(glob, "matchstring")
		if err != nil {
			return fmt.Errorf("parse glob: %w", err)
		}
	}
	return nil
}

// FindFiles gets a list of paths given an array of glob patterns to include and exclude
func FindFiles(includes []string, excludes []string) ([]string, error) {
	var errs error

	allSet := make(map[string]struct{}, len(includes))
	for _, include := range includes {
		matches, err := doublestar.FilepathGlob(include, doublestar.WithFilesOnly(), doublestar.WithFailOnIOErrors())
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("find files with '%s' pattern: %w", include, err))
			// the same pattern could cause an IO error due to one file or directory,
			// but also could still find files without `doublestar.WithFailOnIOErrors()`.
			matches, _ = doublestar.FilepathGlob(include, doublestar.WithFilesOnly())
		}
	INCLUDE:
		for _, match := range matches {
			for _, exclude := range excludes {
				if itMatches, _ := doublestar.PathMatch(exclude, match); itMatches {
					continue INCLUDE
				}
			}

			allSet[match] = struct{}{}
		}
	}

	keys := slices.Collect(maps.Keys(allSet))
	slices.Sort(keys)
	return keys, errs
}
