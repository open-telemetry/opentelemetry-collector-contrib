// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type context struct {
	Name                    string
	Package                 string
	PackagePath             string
	PbPackagePath           string
	PbPackage               string
	LabelType               string
	SampleTimestampField    string
	ExemplarTimestampField  string
	HistogramTimestampField string
	UnknownType             string
	GaugeType               string
	CounterType             string
	HistogramType           string
	SummaryType             string
}

func main() {
	c := context{
		Name:                    "Prometheus",
		Package:                 "prometheusremotewrite",
		PackagePath:             "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite",
		PbPackagePath:           "github.com/prometheus/prometheus/prompb",
		PbPackage:               "prompb",
		LabelType:               "Label",
		SampleTimestampField:    "Timestamp",
		ExemplarTimestampField:  "Timestamp",
		HistogramTimestampField: "Timestamp",
		UnknownType:             "MetricMetadata_UNKNOWN",
		GaugeType:               "MetricMetadata_GAUGE",
		CounterType:             "MetricMetadata_COUNTER",
		HistogramType:           "MetricMetadata_HISTOGRAM",
		SummaryType:             "MetricMetadata_SUMMARY",
	}

	ms, err := filepath.Glob("templates/*.go.tmpl")
	if err != nil {
		panic(err)
	}
	for _, m := range ms {
		t, err := template.ParseFiles(m)
		if err != nil {
			panic(err)
		}

		name, _, found := strings.Cut(filepath.Base(m), ".")
		if !found {
			panic(fmt.Errorf("invalid filename %q", m))
		}
		executeTemplate(t, name, c)
	}
}

func executeTemplate(t *template.Template, name string, c context) {
	f, err := os.Create(fmt.Sprintf("%s_generated.go", name))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if _, err := f.WriteString("// Automatically generated file - do not edit!!\n\n"); err != nil {
		panic(err)
	}

	if err := t.Execute(f, c); err != nil {
		panic(err)
	}
}
