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
