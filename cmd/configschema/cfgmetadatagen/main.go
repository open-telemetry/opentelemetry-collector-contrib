// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/cfgmetadatagen/cfgmetadatagen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func main() {
	sourceDir, outputDir := getFlags()
	c, err := components.Components()
	if err != nil {
		fmt.Printf("error getting components %v", err)
		os.Exit(1)
	}
	err = cfgmetadatagen.GenerateFiles(c, sourceDir, outputDir)
	if err != nil {
		fmt.Printf("cfg metadata generator failed: %v\n", err)
	}
}

func getFlags() (string, string) {
	sourceDir := flag.String("s", filepath.Join("..", ".."), "")
	outputDir := flag.String("o", "cfg-metadata", "output dir")
	flag.Parse()
	return *sourceDir, *outputDir
}
