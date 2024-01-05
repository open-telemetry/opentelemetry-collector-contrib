// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Deprecated: [v0.92.0] This package is deprecated and will be removed in a future release.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"
)

func main() {

	sourceDir, outputDir := getFlags()
	c, err := components.Components()
	if err != nil {
		fmt.Printf("error getting components %v", err)
		os.Exit(1)
	}

	for _, e := range c.Extensions {
		filename := fmt.Sprintf("%s.yaml", e.Type())
		err := configschema.GenerateMetadata(e, sourceDir, outputDir, filename)
		if err != nil {
			fmt.Printf("skipped writing config meta yaml: %v\n", err)
		}
	}
	for _, e := range c.Exporters {
		filename := fmt.Sprintf("%s.yaml", e.Type())
		err := configschema.GenerateMetadata(e, sourceDir, outputDir, filename)
		if err != nil {
			fmt.Printf("skipped writing config meta yaml: %v\n", err)
		}
	}
	for _, p := range c.Processors {
		filename := fmt.Sprintf("%s.yaml", p.Type())
		err := configschema.GenerateMetadata(p, sourceDir, outputDir, filename)
		if err != nil {
			fmt.Printf("skipped writing config meta yaml: %v\n", err)
		}
	}
	for _, r := range c.Receivers {
		filename := fmt.Sprintf("%s.yaml", r.Type())
		err := configschema.GenerateMetadata(r, sourceDir, outputDir, filename)
		if err != nil {
			fmt.Printf("skipped writing config meta yaml: %v\n", err)
		}
	}
	for _, c := range c.Connectors {
		filename := fmt.Sprintf("%s.yaml", c.Type())
		err := configschema.GenerateMetadata(c, sourceDir, outputDir, filename)
		if err != nil {
			fmt.Printf("skipped writing config meta yaml: %v\n", err)
		}
	}
}

func getFlags() (string, string) {
	sourceDir := flag.String("s", filepath.Join("..", ".."), "")
	outputDir := flag.String("o", "cfg-metadata", "output dir")
	flag.Parse()
	return *sourceDir, *outputDir
}
