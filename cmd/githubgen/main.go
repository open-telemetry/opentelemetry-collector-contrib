// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
)

const codeownersHeader = `
#####################################################
#
# List of approvers for OpenTelemetry Collector Contrib
#
#####################################################
#
# Learn about membership in OpenTelemetry community:
#  https://github.com/open-telemetry/community/blob/main/community-membership.md
#
#
# Learn about CODEOWNERS file format:
# https://help.github.com/en/articles/about-code-owners
#
# NOTE: Lines should be entered in the following format:
# <component_path_relative_from_project_root>/<min_1_space><owner_1><space><owner_2><space>..<owner_n>
# extension/oauth2clientauthextension/                 @open-telemetry/collector-contrib-approvers @pavankrish123 @jpkrohling
# Path separator and minimum of 1 space between component path and owners is
# important for validation steps
#

* @open-telemetry/collector-contrib-approvers
`

const allowlistHeader = `
#####################################################
#
# List of components in OpenTelemetry Collector Contrib
# waiting on owners to be assigned
#
#####################################################
#
# Learn about membership in OpenTelemetry community:
#  https://github.com/open-telemetry/community/blob/main/community-membership.md
#
#
# Learn about CODEOWNERS file format:
#  https://help.github.com/en/articles/about-code-owners
#

## 
# NOTE: New components MUST have a codeowner. Add new components to the CODEOWNERS file instead of here.
##

## COMMON & SHARED components
internal/common

`

// Generates files specific to Github according to status metadata:
// .github/CODEOWNERS
// .github/ALLOWLIST
func main() {
	flag.Parse()
	folder := flag.Arg(0)
	if err := run(folder); err != nil {
		log.Fatal(err)
	}
}

type Codeowners struct {
	// Active codeowners
	Active []string `mapstructure:"active"`
	// Emeritus codeowners
	Emeritus []string `mapstructure:"emeritus"`
}
type Status struct {
	Stability     map[string][]string `mapstructure:"stability"`
	Distributions []string            `mapstructure:"distributions"`
	Class         string              `mapstructure:"class"`
	Warnings      []string            `mapstructure:"warnings"`
	Codeowners    *Codeowners         `mapstructure:"codeowners"`
}
type metadata struct {
	// Type of the component.
	Type string `mapstructure:"type"`
	// Type of the parent component (applicable to subcomponents).
	Parent string `mapstructure:"parent"`
	// Status information for the component.
	Status *Status `mapstructure:"status"`
}

func loadMetadata(filePath string) (metadata, error) {
	cp, err := fileprovider.New().Retrieve(context.Background(), "file:"+filePath, nil)
	if err != nil {
		return metadata{}, err
	}

	conf, err := cp.AsConf()
	if err != nil {
		return metadata{}, err
	}

	md := metadata{}
	if err := conf.Unmarshal(&md); err != nil {
		return md, err
	}

	return md, nil
}

func run(folder string) error {
	components := map[string]metadata{}
	foldersList := []string{}
	maxLength := 0
	err := filepath.Walk(folder, func(path string, info fs.FileInfo, err error) error {
		if info.Name() == "metadata.yaml" {
			m, err := loadMetadata(path)
			if err != nil {
				return err
			}
			if m.Status == nil {
				return nil
			}
			key := filepath.Dir(path) + "/"
			components[key] = m
			foldersList = append(foldersList, key)
			for stability, _ := range m.Status.Stability {
				if stability == "unmaintained" {
					// do not account for unmaintained status to change the max length of the component line.
					return nil
				}
			}
			if len(key) > maxLength {
				maxLength = len(key)
			}
		}
		return nil
	})
	sort.Strings(foldersList)
	if err != nil {
		return err
	}
	codeowners := codeownersHeader
	deprecatedList := "## DEPRECATED components\n"
	unmaintainedList := "\n## UNMAINTAINED components\n"

	currentFirstSegment := ""
LOOP:
	for _, key := range foldersList {
		m := components[key]
		for stability, _ := range m.Status.Stability {
			if stability == "unmaintained" {
				unmaintainedList += key + "\n"
				continue LOOP
			}
			if stability == "deprecated" {
				deprecatedList += key + "\n"
			}
		}

		if m.Status.Codeowners != nil {
			parts := strings.Split(key, string(os.PathSeparator))
			firstSegment := parts[0]
			if firstSegment != currentFirstSegment {
				currentFirstSegment = firstSegment
				codeowners += "\n"
			}
			owners := ""
			for i, owner := range m.Status.Codeowners.Active {
				if i > 0 {
					owners += " "
				}
				owners += "@" + owner
			}
			codeowners += fmt.Sprintf("%s%s @open-telemetry/collector-contrib-approvers %s\n", key, strings.Repeat(" ", maxLength-len(key)), owners)
		}
	}

	err = os.WriteFile(filepath.Join(".github", "CODEOWNERS"), []byte(codeowners), 0644)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(".github", "ALLOWLIST"), []byte(allowlistHeader+deprecatedList+unmaintainedList), 0644)
	if err != nil {
		return err
	}

	return nil
}
