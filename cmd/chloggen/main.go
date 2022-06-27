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
	changelogMD = "CHANGELOG.md"
	chlogDir    = "changelog"
	exampleYAML = "EXAMPLE.yaml"
	tmpMD       = "changelog.tmp.MD"

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

	updateCmd := flag.NewFlagSet("preview", flag.ExitOnError)
	version := updateCmd.String("version", "vTODO", "'version' will be rendered directly into the update text")
	dry := updateCmd.Bool("dry", false, "dry will generate the update text and print to stdout")

	switch command := os.Args[1]; command {
	case "validate":
		if err := validate(); err != nil {
			fmt.Printf("FAIL: validate: %v\n", err)
			os.Exit(1)
		}
	case "update":
		updateCmd.Parse(os.Args[2:])
		if err := update(*version, *dry); err != nil {
			fmt.Printf("FAIL: update: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("usage: chloggen validate")
	fmt.Println("       chloggen update [-version v0.55.0] [-dry]")
}

func validate() error {
	entries, err := readEntries(false)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = entry.Validate(); err != nil {
			return err
		}
	}
	fmt.Printf("PASS: all files in ./%s/ are valid\n", chlogDir)
	return nil
}

func update(version string, dry bool) error {
	entries, err := readEntries(true)
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

	oldChlogBytes, err := os.ReadFile(changelogMD)
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

	tmpChlogFile, err := os.Create(tmpMD)
	if err != nil {
		return err
	}
	defer tmpChlogFile.Close()

	tmpChlogFile.WriteString(chlogBuilder.String())

	if err := os.Rename(tmpMD, changelogMD); err != nil {
		return err
	}

	fmt.Printf("Finished updating %s\n", changelogMD)

	return deleteEntries()
}

func readEntries(excludeExample bool) ([]*Entry, error) {
	entryFiles, err := filepath.Glob(filepath.Join(chlogDir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	entries := make([]*Entry, 0, len(entryFiles))
	for _, entryFile := range entryFiles {
		if excludeExample && filepath.Base(entryFile) == exampleYAML {
			continue
		}

		fileBytes, err := os.ReadFile(entryFile)
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

func deleteEntries() error {
	entryFiles, err := filepath.Glob(filepath.Join(chlogDir, "*.yaml"))
	if err != nil {
		return err
	}

	for _, entryFile := range entryFiles {
		if filepath.Base(entryFile) == exampleYAML {
			continue
		}

		if err := os.Remove(entryFile); err != nil {
			fmt.Printf("Failed to delete: %s\n", entryFile)
		}
	}
	return nil
}
