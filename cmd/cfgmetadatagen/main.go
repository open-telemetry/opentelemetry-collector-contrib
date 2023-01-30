// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/cfgschema"
)

func main() {
	err := run()
	if err != nil {
		fmt.Printf("error running command: %s\n", err)
	}
}

func run() error {
	outputType, sourceDir, outputDir := getFlags()
	c, err := components.Components()
	if err != nil {
		return err
	}
	const module = "github.com/open-telemetry/opentelemetry-collector-contrib"
	switch outputType {
	case "yaml":
		return cfgschema.GenerateYAMLFiles(c, sourceDir, outputDir, module)
	case "md":
		return cfgschema.GenerateMDFiles(c, sourceDir, outputDir, module)
	default:
		return fmt.Errorf("unrecognized output type: %s", outputType)
	}
}

func getFlags() (string, string, string) {
	output := flag.String("o", "yaml", `output type: "yaml" or "md"`)
	sourceDir := flag.String("s", filepath.Join("..", ".."), "")
	outputDir := flag.String("d", "cfg-metadata", "output dir")
	flag.Parse()
	return *output, *sourceDir, *outputDir
}
