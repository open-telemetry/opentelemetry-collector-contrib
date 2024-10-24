// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package finder // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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

type BaseGroups map[string][]string

var metaReplacer = strings.NewReplacer("\\*", "*", "\\?", "?", "\\[", "[", "\\]", "]", "\\{", "{", "\\}", "}")

// GroupByFS preprocesses patterns to allow more performant usage in the hot path.
// Specifically, it cleans each pattern, then splits it into a base and a file part.
// Then it groups patterns by base so that later we can direclty create a single fs.FS.
//
// Code is modified from the upstream doublestar.FilepathGlob function.
// See https://github.com/bmatcuk/doublestar/blob/1e20c6dbac3e865d289d5edb5e18368fbc897c5c/utils.go#L97C1-L120C22
func GroupByBase(patterns []string) (BaseGroups, error) {
	patternsByBase := make(map[string][]string)

	for _, pattern := range patterns {
		pattern = filepath.Clean(pattern)
		pattern = filepath.ToSlash(pattern)
		base, relPattern := doublestar.SplitPattern(pattern)

		if relPattern == "" || relPattern == "." || relPattern == ".." {
			// some special cases to match filepath.Glob behavior
			if !doublestar.ValidatePathPattern(pattern) {
				return nil, doublestar.ErrBadPattern
			}

			if filepath.Separator != '\\' {
				relPattern = metaReplacer.Replace(relPattern)
			}

			if _, err := os.Lstat(pattern); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue // behave as without doublestar.WithFailOnPatternNotExist()
				}
				return nil, err // behave as with doublestar.WithFailOnIOErrors()
			}
		}

		if _, ok := patternsByBase[base]; !ok {
			patternsByBase[base] = make([]string, 0)
		}
		patternsByBase[base] = append(patternsByBase[base], filepath.FromSlash(relPattern))
	}

	return patternsByBase, nil
}

func FindFiles(includes BaseGroups, excludes []string) (result []string, errs error) {
	excludeCache := make(map[string]bool) // map[path]alreadyExcluded
	for base, patterns := range includes {
		baseFS := os.DirFS(base)
		for _, pattern := range patterns {
			globWalk := func(path string, d fs.DirEntry) error {
				path = filepath.Join(base, path) // prepend the base so we're matching against the full path

				// When pulling a value from the map:
				// The first bool represents the actual value in the map, which is only true if we've previously determined the path should be excluded.
				// The second bool represents whether the path previously was seen at all. If it was, and we already determined whether to exclude or accept it.
				if previouslyExcluded, previouslySeen := excludeCache[path]; previouslySeen || previouslyExcluded {
					return nil
				}

				// Haven't seen this path yet, check if it should be excluded.
				for _, exclude := range excludes {
					if doublestar.PathMatchUnvalidated(exclude, path) {
						excludeCache[path] = true
						return nil
					}
				}
				excludeCache[path] = false
				result = append(result, path)
				return nil
			}

			err := doublestar.GlobWalk(baseFS, pattern, globWalk, doublestar.WithFilesOnly(), doublestar.WithFailOnIOErrors())
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("find files with '%s' pattern: %w", filepath.Join(base, pattern), err))
				// the same pattern could cause an IO error due to one file or directory,
				// but also could still find files without `doublestar.WithFailOnIOErrors()`.
				doublestar.GlobWalk(baseFS, pattern, globWalk, doublestar.WithFilesOnly())
			}
		}
	}
	return
}

// return true to include the file, false to exclude it
type GlobWalkFunc func(path string, d fs.DirEntry) bool
