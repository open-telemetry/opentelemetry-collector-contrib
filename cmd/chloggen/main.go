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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	breaking     = "breaking"
	deprecation  = "deprecation"
	newComponent = "new_component"
	enhancement  = "enhancement"
	bugFix       = "bug_fix"

	insertPoint = "<!-- next version -->"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	newCmd := flag.NewFlagSet("new", flag.ExitOnError)
	filename := newCmd.String("filename", "", "'filename' is required")

	updateCmd := flag.NewFlagSet("preview", flag.ExitOnError)
	version := updateCmd.String("version", "vTODO", "'version' will be rendered directly into the update text")
	dry := updateCmd.Bool("dry", false, "'dry' will generate the update text and print to stdout")

	switch command := os.Args[1]; command {
	case "new":
		if err := newCmd.Parse(os.Args[2:]); err != nil {
			fmt.Printf("FAIL: new: %v\n", err)
			os.Exit(1)
		}
		if err := initialize(defaultCtx, *filename); err != nil {
			fmt.Printf("FAIL: new: %v\n", err)
			os.Exit(1)
		}
	case "validate":
		if err := validate(defaultCtx); err != nil {
			fmt.Printf("FAIL: validate: %v\n", err)
			os.Exit(1)
		}
	case "update":
		if err := updateCmd.Parse(os.Args[2:]); err != nil {
			fmt.Printf("FAIL: update: %v\n", err)
			os.Exit(1)
		}
		if err := update(defaultCtx, *version, *dry); err != nil {
			fmt.Printf("FAIL: update: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func initialize(ctx chlogContext, filename string) error {
	path := filepath.Join(ctx.unreleasedDir, filename)
	var pathWithExt string
	switch ext := filepath.Ext(path); ext {
	case ".yaml":
		pathWithExt = path
	case ".yml":
		pathWithExt = strings.TrimSuffix(path, ".yml") + ".yaml"
	case "":
		pathWithExt = path + ".yaml"
	default:
		return fmt.Errorf("non-yaml extension: %s", ext)
	}

	templateBytes, err := os.ReadFile(defaultCtx.templateYAML)
	if err != nil {
		return err
	}
	err = os.WriteFile(pathWithExt, templateBytes, os.FileMode(0755))
	if err != nil {
		return err
	}
	fmt.Printf("Changelog entry template copied to: %s\n", pathWithExt)
	return nil
}

func validate(ctx chlogContext) error {
	entries, err := readEntries(ctx)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = entry.Validate(); err != nil {
			return err
		}
	}
	fmt.Printf("PASS: all files in ./%s/ are valid\n", ctx.unreleasedDir)
	return nil
}

func update(ctx chlogContext, version string, dry bool) error {
	entries, err := readEntries(ctx)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("no entries to add to the changelog")
	}

	chlogUpdate, err := generateSummary(version, entries)
	if err != nil {
		return err
	}

	if dry {
		fmt.Printf("Generated changelog updates:")
		fmt.Println(chlogUpdate)
		return nil
	}

	oldChlogBytes, err := os.ReadFile(ctx.changelogMD)
	if err != nil {
		return err
	}
	chlogParts := bytes.Split(oldChlogBytes, []byte(insertPoint))
	if len(chlogParts) != 2 {
		return fmt.Errorf("expected one instance of %s", insertPoint)
	}

	chlogHeader, chlogHistory := string(chlogParts[0]), string(chlogParts[1])

	var chlogBuilder strings.Builder
	chlogBuilder.WriteString(chlogHeader)
	chlogBuilder.WriteString(insertPoint)
	chlogBuilder.WriteString(chlogUpdate)
	chlogBuilder.WriteString(chlogHistory)

	tmpMD := ctx.changelogMD + ".tmp"
	tmpChlogFile, err := os.Create(tmpMD)
	if err != nil {
		return err
	}
	if _, err := tmpChlogFile.WriteString(chlogBuilder.String()); err != nil {
		return err
	}
	if err := tmpChlogFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpMD, ctx.changelogMD); err != nil {
		return err
	}

	fmt.Printf("Finished updating %s\n", ctx.changelogMD)

	return deleteEntries(ctx)
}

func readEntries(ctx chlogContext) ([]*Entry, error) {
	entryYAMLs, err := filepath.Glob(filepath.Join(ctx.unreleasedDir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	entries := make([]*Entry, 0, len(entryYAMLs))
	for _, entryYAML := range entryYAMLs {
		if filepath.Base(entryYAML) == filepath.Base(ctx.templateYAML) {
			continue
		}

		fileBytes, err := os.ReadFile(entryYAML)
		if err != nil {
			return nil, err
		}

		entry := &Entry{}
		if err = yaml.Unmarshal(fileBytes, entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func deleteEntries(ctx chlogContext) error {
	entryYAMLs, err := filepath.Glob(filepath.Join(ctx.unreleasedDir, "*.yaml"))
	if err != nil {
		return err
	}

	for _, entryYAML := range entryYAMLs {
		if filepath.Base(entryYAML) == filepath.Base(ctx.templateYAML) {
			continue
		}

		if err := os.Remove(entryYAML); err != nil {
			fmt.Printf("Failed to delete: %s\n", entryYAML)
		}
	}
	return nil
}

func usage() {
	fmt.Println("usage: [FILENAME=my-change] chloggen new")
	fmt.Println("       chloggen validate")
	fmt.Println("       chloggen update [-version v0.55.0] [-dry]")
}
