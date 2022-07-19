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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  string
	}{
		{
			name:     "no_extension",
			filename: "my-change",
		},
		{
			name:     "yaml_extension",
			filename: "some-change.yaml",
		},
		{
			name:     "yml_extension",
			filename: "some-change.yml",
		},
		{
			name:     "replace_forward_slash",
			filename: "replace/forward/slash",
		},
		{
			name:     "bad_extension",
			filename: "my-change.txt",
			wantErr:  "non-yaml extension",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupTestDir(t, []*Entry{})
			err := initialize(ctx, tc.filename)
			if tc.wantErr != "" {
				require.Regexp(t, tc.wantErr, err)
				return
			}
			require.NoError(t, err)

			require.Error(t, validate(ctx), "The new entry should not be valid without user input")
		})
	}
}

func TestValidateE2E(t *testing.T) {
	tests := []struct {
		name    string
		entries []*Entry
		wantErr string
	}{
		{
			name:    "all_valid",
			entries: getSampleEntries(),
		},
		{
			name: "invalid_change_type",
			entries: func() []*Entry {
				return append(getSampleEntries(), &Entry{
					ChangeType: "fake",
					Component:  "receiver/foo",
					Note:       "Add some bar",
					Issues:     []int{12345},
				})
			}(),
			wantErr: "'fake' is not a valid 'change_type'",
		},
		{
			name: "missing_component",
			entries: func() []*Entry {
				return append(getSampleEntries(), &Entry{
					ChangeType: bugFix,
					Component:  "",
					Note:       "Add some bar",
					Issues:     []int{12345},
				})
			}(),
			wantErr: "specify a 'component'",
		},
		{
			name: "missing_note",
			entries: func() []*Entry {
				return append(getSampleEntries(), &Entry{
					ChangeType: bugFix,
					Component:  "receiver/foo",
					Note:       "",
					Issues:     []int{12345},
				})
			}(),
			wantErr: "specify a 'note'",
		},
		{
			name: "missing_issue",
			entries: func() []*Entry {
				return append(getSampleEntries(), &Entry{
					ChangeType: bugFix,
					Component:  "receiver/foo",
					Note:       "Add some bar",
					Issues:     []int{},
				})
			}(),
			wantErr: "specify one or more issues #'s",
		},
		{
			name: "all_invalid",
			entries: func() []*Entry {
				sampleEntries := getSampleEntries()
				for _, e := range sampleEntries {
					e.ChangeType = "fake"
				}
				return sampleEntries
			}(),
			wantErr: "'fake' is not a valid 'change_type'",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupTestDir(t, tc.entries)

			err := validate(ctx)
			if tc.wantErr != "" {
				require.Regexp(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateE2E(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows line breaks cause comparison failures w/ golden files.")
	}
	tests := []struct {
		name    string
		entries []*Entry
		version string
		dry     bool
	}{
		{
			name:    "all_change_types",
			entries: getSampleEntries(),
			version: "v0.45.0",
		},
		{
			name:    "all_change_types_multiple",
			entries: append(getSampleEntries(), getSampleEntries()...),
			version: "v0.45.0",
		},
		{
			name:    "dry_run",
			entries: getSampleEntries(),
			version: "v0.45.0",
			dry:     true,
		},
		{
			name:    "deprecation_only",
			entries: []*Entry{deprecationEntry()},
			version: "v0.45.0",
		},
		{
			name:    "new_component_only",
			entries: []*Entry{newComponentEntry()},
			version: "v0.45.0",
		},
		{
			name:    "bug_fix_only",
			entries: []*Entry{bugFixEntry()},
			version: "v0.45.0",
		},
		{
			name:    "enhancement_only",
			entries: []*Entry{enhancementEntry()},
			version: "v0.45.0",
		},
		{
			name:    "breaking_only",
			entries: []*Entry{breakingEntry()},
			version: "v0.45.0",
		},
		{
			name:    "subtext",
			entries: []*Entry{entryWithSubtext()},
			version: "v0.45.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupTestDir(t, tc.entries)

			require.NoError(t, update(ctx, tc.version, tc.dry))

			actualBytes, err := os.ReadFile(ctx.changelogMD)
			require.NoError(t, err)

			expectedChangelogMD := filepath.Join("testdata", tc.name+".md")
			expectedBytes, err := os.ReadFile(expectedChangelogMD)
			require.NoError(t, err)

			require.Equal(t, string(expectedBytes), string(actualBytes))

			remainingYAMLs, err := filepath.Glob(filepath.Join(ctx.unreleasedDir, "*.yaml"))
			require.NoError(t, err)
			if tc.dry {
				require.Equal(t, 1+len(tc.entries), len(remainingYAMLs))
			} else {
				require.Equal(t, 1, len(remainingYAMLs))
				require.Equal(t, ctx.templateYAML, remainingYAMLs[0])
			}
		})
	}
}

func getSampleEntries() []*Entry {
	return []*Entry{
		enhancementEntry(),
		bugFixEntry(),
		deprecationEntry(),
		newComponentEntry(),
		breakingEntry(),
		entryWithSubtext(),
	}
}

func enhancementEntry() *Entry {
	return &Entry{
		ChangeType: enhancement,
		Component:  "receiver/foo",
		Note:       "Add some bar",
		Issues:     []int{12345},
	}
}

func bugFixEntry() *Entry {
	return &Entry{
		ChangeType: bugFix,
		Component:  "testbed",
		Note:       "Fix blah",
		Issues:     []int{12346, 12347},
	}
}

func deprecationEntry() *Entry {
	return &Entry{
		ChangeType: deprecation,
		Component:  "exporter/old",
		Note:       "Deprecate old",
		Issues:     []int{12348},
	}
}

func newComponentEntry() *Entry {
	return &Entry{
		ChangeType: newComponent,
		Component:  "exporter/new",
		Note:       "Add new exporter ...",
		Issues:     []int{12349},
	}
}

func breakingEntry() *Entry {
	return &Entry{
		ChangeType: breaking,
		Component:  "processor/oops",
		Note:       "Change behavior when ...",
		Issues:     []int{12350},
	}
}

func entryWithSubtext() *Entry {
	lines := []string{"- foo\n  - bar\n- blah\n  - 1234567"}

	return &Entry{
		ChangeType: breaking,
		Component:  "processor/oops",
		Note:       "Change behavior when ...",
		Issues:     []int{12350},
		SubText:    strings.Join(lines, "\n"),
	}
}

func setupTestDir(t *testing.T, entries []*Entry) chlogContext {
	ctx := newChlogContext(t.TempDir())

	// Create a known dummy changelog which may be updated by the test
	changelogBytes, err := os.ReadFile(filepath.Join("testdata", changelogMD))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ctx.changelogMD, changelogBytes, os.FileMode(0755)))

	require.NoError(t, os.Mkdir(ctx.unreleasedDir, os.FileMode(0755)))

	// Copy the entry template, for tests that ensure it is not deleted
	templateInRootDir := defaultCtx.templateYAML
	templateBytes, err := os.ReadFile(templateInRootDir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ctx.templateYAML, templateBytes, os.FileMode(0755)))

	for i, entry := range entries {
		require.NoError(t, writeEntryYAML(ctx, fmt.Sprintf("%d.yaml", i), entry))
	}

	return ctx
}

func writeEntryYAML(ctx chlogContext, filename string, entry *Entry) error {
	entryBytes, err := yaml.Marshal(entry)
	if err != nil {
		return err
	}
	path := filepath.Join(ctx.unreleasedDir, filename)
	return os.WriteFile(path, entryBytes, os.FileMode(0755))
}

func TestCleanFilename(t *testing.T) {
	require.Equal(t, "fix_some_bug", cleanFileName("fix/some_bug"))
	require.Equal(t, "fix_some_bug", cleanFileName("fix\\some_bug"))
}
