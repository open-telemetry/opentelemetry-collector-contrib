// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Program otelcontribcol is an extension to the OpenTelemetry Collector
// that includes additional components, some vendor-specific, contributed
// from the wider community.
package main

import (
	"fmt"
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/service"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/version"
)

func main() {
	factories, err := components.Components()
	if err != nil {
		log.Fatalf("failed to build components: %v", err)
	}

	info := component.BuildInfo{
		Command:     "otelcontribcol",
		Description: "OpenTelemetry Collector Contrib",
		Version:     version.Version,
	}

	if err = run(service.CollectorSettings{BuildInfo: info, Factories: factories}); err != nil {
		log.Fatal(err)
	}
}

func runInteractive(params service.CollectorSettings) error {
	cmd := service.NewCommand(params)
	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("collector server run finished with error: %w", err)
	}

	return nil
}
