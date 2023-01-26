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
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/cfgmetadatagen/cfgmetadatagen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/internal/components"
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
