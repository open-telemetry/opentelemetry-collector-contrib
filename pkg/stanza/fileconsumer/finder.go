// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"github.com/bmatcuk/doublestar/v4"
)

type Finder struct {
	Include []string `mapstructure:"include,omitempty"`
	Exclude []string `mapstructure:"exclude,omitempty"`
}

// FindFiles gets a list of paths given an array of glob patterns to include and exclude
//
// Deprecated: This will be made internal in a future release.
func (f Finder) FindFiles() []string {
	all := make([]string, 0, len(f.Include))
	for _, include := range f.Include {
		matches, _ := doublestar.FilepathGlob(include, doublestar.WithFilesOnly()) // compile error checked in build
	INCLUDE:
		for _, match := range matches {
			for _, exclude := range f.Exclude {
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

	return all
}
