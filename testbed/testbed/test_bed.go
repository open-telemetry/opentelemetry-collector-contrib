// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package testbed allows to easily set up a test that requires running the agent
// and a load generator, measure and define resource consumption expectations
// for the agent, fail tests automatically when expectations are exceeded.
//
// Each test case requires a agent configuration file and (optionally) load
// generator spec file. Test cases are defined as regular Go tests.
//
// Agent and load generator must be pre-built and their paths must be specified in
// test bed config file. RUN_TESTBED env variable must be defined for tests to run.
package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func Start(resultsSummary TestResultsSummary) error {
	dir, err := filepath.Abs("results")
	if err != nil {
		log.Fatal(err.Error())
	}
	resultsSummary.Init(dir)

	return err
}

func SaveResults(resultsSummary TestResultsSummary) {
	resultsSummary.Save()
}

const testBedEnableEnvVarName = "RUN_TESTBED"

// GlobalConfig global config for testbed.
var GlobalConfig = struct {
	// DefaultAgentExeRelativeFile relative path to default agent executable to test.
	// Can be set in the contrib repo to use a different executable name.
	// Set this before calling DoTestMain().
	//
	// If used in the path, {{.GOOS}} and {{.GOARCH}} will be expanded to the current
	// OS and ARCH correspondingly.
	//
	// Individual tests can override this by setting the AgentExePath of childProcessCollector
	// that is passed to the TestCase.
	DefaultAgentExeRelativeFile string
}{
	// DefaultAgentExeRelativeFile the default exe that is produced by Makefile "otelcol" target relative
	// to testbed/tests directory.
	DefaultAgentExeRelativeFile: "../../bin/oteltestbedcol_{{.GOOS}}_{{.GOARCH}}",
}

// DoTestMain is intended to be run from TestMain somewhere in the test suit.
// This enables the testbed.
func DoTestMain(m *testing.M, resultsSummary TestResultsSummary) {
	testBedConfigFile := os.Getenv(testBedEnableEnvVarName)
	if testBedConfigFile == "" {
		log.Printf(testBedEnableEnvVarName + " is not defined, skipping E2E tests.")
		os.Exit(0)
	}

	// Load the test bed config first.
	err := Start(resultsSummary)
	if err != nil {
		log.Fatal(err.Error())
	}

	res := m.Run()

	SaveResults(resultsSummary)

	// Now run all tests.
	os.Exit(res)
}
