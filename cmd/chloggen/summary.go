// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"text/template"
)

type summary struct {
	Version         string
	BreakingChanges []string
	Deprecations    []string
	NewComponents   []string
	Enhancements    []string
	BugFixes        []string
}

func generateSummary(version string, entries []*Entry) (string, error) {
	s := summary{
		Version: version,
	}

	for _, entry := range entries {
		switch entry.ChangeType {
		case breaking:
			s.BreakingChanges = append(s.BreakingChanges, entry.String())
		case deprecation:
			s.Deprecations = append(s.Deprecations, entry.String())
		case newComponent:
			s.NewComponents = append(s.NewComponents, entry.String())
		case enhancement:
			s.Enhancements = append(s.Enhancements, entry.String())
		case bugFix:
			s.BugFixes = append(s.BugFixes, entry.String())
		}
	}

	s.BreakingChanges = sort.StringSlice(s.BreakingChanges)
	s.Deprecations = sort.StringSlice(s.Deprecations)
	s.NewComponents = sort.StringSlice(s.NewComponents)
	s.Enhancements = sort.StringSlice(s.Enhancements)
	s.BugFixes = sort.StringSlice(s.BugFixes)

	return s.String()
}

func (s summary) String() (string, error) {
	summaryTmpl := filepath.Join(thisDir(), "summary.tmpl")

	tmpl := template.Must(
		template.
			New("summary.tmpl").
			Option("missingkey=error").
			ParseFiles(summaryTmpl))

	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed executing template: %w", err)
	}

	return buf.String(), nil
}
