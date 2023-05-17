// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/metrics"

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"
)

type results struct {
	resultsFile *os.File
}

type result struct {
	testName   string
	testResult string
	numDiffs   int
}

func (r *results) Init(resultsDir string) {
	err := os.MkdirAll(resultsDir, os.FileMode(0755))
	if err != nil {
		log.Fatalf(err.Error())
	}
	r.resultsFile, err = os.Create(path.Join(resultsDir, "TESTRESULTS.md"))
	if err != nil {
		log.Fatalf(err.Error())
	}
	r.writeString(
		"# Test Results\n" +
			fmt.Sprintf("Started: %s\n\n", time.Now().Format(time.RFC1123Z)) +
			"Test                                    |Result|Num Diffs|\n" +
			"----------------------------------------|------|--------:|\n",
	)
}

func (r *results) Add(_ string, rslt interface{}) {
	tr := rslt.(result)
	line := fmt.Sprintf(
		"%-40s|%-6s|%9d|\n",
		tr.testName,
		tr.testResult,
		tr.numDiffs,
	)
	r.writeString(line)
}

func (r *results) writeString(s string) {
	_, _ = io.WriteString(r.resultsFile, s)
}

func (r *results) Save() {
	err := r.resultsFile.Close()
	if err != nil {
		panic(err)
	}
}
