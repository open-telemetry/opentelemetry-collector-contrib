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

//go:build ignore
// +build ignore

// TODO: Only supports Metrics Sum & Gauge. To implement more, update the "readMetrics" function
// from internal/testutil/stdout.go.
//
// For additional telemetry, update ReadLoggingExporter and the endpoints in this file.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/kr/pretty"
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
)

const pathSeries = "/api/v1/series"

func main() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fileConfig := flag.String("c", "", "Path to the collector config file; used for configuring the exporter. If left empty, default configuration will be used")
	fileLog := flag.String("l", "", "Collector log file output to use for input")
	v := flag.Bool("v", false, "verbose")
	typ := flag.String("t", "json", "Output type: json or points")
	flag.Parse()

	if *fileLog == "" {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	// maps URLs to consecutive request bodies
	payloads := make(map[string][][]byte)

	server := testutil.DatadogServerMock(
		testutil.OverwriteHandleFunc(func() (string, http.HandlerFunc) {
			return pathSeries, func(w http.ResponseWriter, req *http.Request) {
				slurp, _ := io.ReadAll(req.Body)
				payloads[pathSeries] = append(payloads[pathSeries], slurp)
			}
		}),
	)
	defer server.Close()

	var exporters struct {
		Datadog *datadogexporter.Config `mapstructure:"datadog"`
	}
	fact := datadogexporter.NewFactory()
	exporters.Datadog = fact.CreateDefaultConfig().(*datadogexporter.Config)
	if *fileConfig != "" {
		// if a config file was specified as a command line argument, merge it
		// with the default configuration
		slurp, err := os.ReadFile(*fileConfig)
		if err != nil {
			log.Fatal(fmt.Errorf("error reading config file: %w", err))
		}
		var all map[string]interface{}
		if err := yaml.Unmarshal(slurp, &all); err != nil {
			log.Fatal(fmt.Errorf("error unmarshalling config file: %w", err))
		}
		if err := mapstructure.Decode(all["exporters"], &exporters); err != nil {
			log.Fatal(fmt.Errorf("error merging config: %w", err))
		}
	}
	exporters.Datadog.API.FailOnInvalidKey = false // we don't care about an accurate one
	exporters.Datadog.Metrics.TCPAddr.Endpoint = server.URL

	ctx := context.Background()
	set := componenttest.NewNopExporterCreateSettings()
	if *v {
		logger := zap.Must(zap.NewDevelopment())
		defer logger.Sync()
		set.TelemetrySettings.Logger = logger
	}
	me, err := fact.CreateMetricsExporter(ctx, set, exporters.Datadog)
	if err != nil {
		log.Fatal(fmt.Errorf("error creating metrics exporter: %w", err))
	}

	f, err := os.Open(*fileLog)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	pd, err := testutil.ReadLoggingExporter(f)
	if err != nil {
		log.Fatal(err)
	}
	if *v {
		fmt.Printf("Consuming %d metrics.", len(pd.Metrics))
	}
	for _, mm := range pd.Metrics {
		if *v {
			fmt.Print(".")
		}
		if err := me.ConsumeMetrics(ctx, mm); err != nil {
			log.Fatal(err)
		}
	}
	for _, p := range payloads[pathSeries] {
		if *typ == "json" {
			var out map[string]interface{}
			if err := json.Unmarshal(p, &out); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%# v", pretty.Formatter(out))
		} else {
			var out struct {
				Series []datadog.Metric `json:"series"`
			}
			if err := json.Unmarshal(p, &out); err != nil {
				log.Fatal(err)
			}
			for _, s := range out.Series {
				if strings.HasPrefix(*s.Metric, "otel.") || strings.HasPrefix(*s.Metric, "system.") {
					continue
				}
				fmt.Printf("%s: ", *s.Metric)
				for _, dp := range s.Points {
					fmt.Printf("[%f, %f]", *dp[0], *dp[1])
				}
				fmt.Println()
			}
		}
	}
}
