// Copyright OpenTelemetry Authors
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
	"context"
	"errors"
	"flag"
	"fmt"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	flag.Parse()
	yml := flag.Arg(0)
	if err := run(yml); err != nil {
		log.Fatal(err)
	}
}

func run(ymlPath string) error {
	if ymlPath == "" {
		return errors.New("argument must be metadata.yaml file")
	}
	ymlDir, err := filepath.Abs(ymlPath)
	if err != nil {
		return fmt.Errorf("cannot compute the absolute path of %s", ymlDir)
	}
	ymlDir = path.Dir(ymlDir)
	if _, err := os.Stat(ymlPath); os.IsNotExist(err) {
		log.Printf("metadata.yaml missing for %s, skipping\n", ymlDir)
		return nil
	}

	md, err := loadMetadata(filepath.Clean(ymlPath))
	if err != nil {
		return fmt.Errorf("failed loading %v: %w", ymlPath, err)
	}
	if md.Status == nil {
		return errors.New("missing status metadata")
	}
	if md.Type == "" {
		return errors.New("missing type metadata")
	}
	if md.Status.Class == "" {
		return errors.New("missing status class metadata")
	}
	if md.Status.Stability == "" {
		return errors.New("missing status stability metadata")
	}
	switch md.Status.Stability {
	case "development":
		log.Println("component in development, skipping checks")
		return nil
	case "alpha", "beta", "stable", "deprecated":
	default:
		return fmt.Errorf("invalid stability level: %s", md.Status.Stability)
	}

	builderConfigYaml, err := os.ReadFile(filepath.Join(ymlDir, "..", "..", "cmd", "otelcontribcol", "builder-config.yaml"))
	if err != nil {
		return err
	}

	lookingForMatch := fmt.Sprintf("github.com/open-telemetry/opentelemetry-collector-contrib/%s/%s", md.Status.Class, path.Base(ymlDir))
	if !strings.Contains(string(builderConfigYaml), lookingForMatch) {
		return fmt.Errorf("%s%s is missing from cmd/otelcontribcol/builder-config.yaml", md.Type, md.Status.Class)
	}
	return nil
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

type metadata struct {
	// Type of the component.
	Type string `mapstructure:"type"`
	// Status information for the component.
	Status *status `mapstructure:"status"`
}

type status struct {
	Stability     string   `mapstructure:"stability"`
	Pipelines     []string `mapstructure:"pipelines"`
	Distributions []string `mapstructure:"distributions"`
	Class         string   `mapstructure:"class"`
	Warnings      []string `mapstructure:"warnings"`
}
