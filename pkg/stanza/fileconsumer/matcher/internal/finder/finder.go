// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package finder // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher/internal/finder"

import (
    "errors"
    "fmt"
    "os"
    "time"

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

// FindFiles gets a list of paths given an array of glob patterns to include and exclude
func FindFiles(includes []string, excludes []string, maxAge time.Duration) ([]string, []string, error) {
    var errs error
    all := make([]string, 0, len(includes))
    for _, include := range includes {

        matches, err := doublestar.FilepathGlob(include, doublestar.WithFilesOnly(), doublestar.WithFailOnIOErrors())
        if err != nil {
            errs = errors.Join(errs, fmt.Errorf("find files with '%s' pattern: %w", include, err))
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

            for _, existing := range all {
                if existing == match {
                    continue INCLUDE
                }
            }

            all = append(all, match)
        }
    }

    recent, old := FilterByMaxAge(all, maxAge)
    return recent, old, errs
}

func FilterByMaxAge(paths []string, maxAge time.Duration) (recent []string, old []string) {
    now := time.Now()
    for _, path := range paths {
        info, statErr := os.Stat(path)
        if statErr != nil {
            recent = append(recent, path)
            continue
        }
        if maxAge > 0 && now.Sub(info.ModTime()) > maxAge {
            old = append(old, path)
        } else {
            recent = append(recent, path)
        }
    }
    return recent, old
}