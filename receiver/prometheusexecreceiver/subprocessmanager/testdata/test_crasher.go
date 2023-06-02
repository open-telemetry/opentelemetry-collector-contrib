// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strconv"
	"time"
)

const defaultSleepTime = 2
const defaultExitCode = 0

// This program is simply a test program that does nothing but crash after a certain time, with a non-zero exit code, used in
// subprocessmanager tests
func main() {

	sleepTime, err := strconv.Atoi(os.Args[1])
	if err != nil {
		sleepTime = defaultSleepTime
	}

	exitCode, err := strconv.Atoi(os.Args[2])
	if err != nil {
		exitCode = defaultExitCode
	}

	time.Sleep(time.Millisecond * time.Duration(sleepTime))
	os.Exit(exitCode)

}
