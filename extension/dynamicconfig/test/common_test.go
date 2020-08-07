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

// +build integration

package test

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

var (
	sec1Schedule = `ConfigBlocks:
    Schedules:
    - InclusionPatterns:
        - StartsWith: "*"
      Period: SEC_1`

	sec5Schedule = `ConfigBlocks:
    Schedules:
    - InclusionPatterns:
        - StartsWith: "*"
      Period: SEC_5`
)

func startCollector(t *testing.T, configPath string, metricsAddr string) (*exec.Cmd, io.ReadCloser) {
	cmdPath := fmt.Sprintf("../../../bin/otelcontribcol_%s_%s",
		runtime.GOOS,
		runtime.GOARCH)
	cmd := exec.Command(cmdPath, "--config", configPath, "--metrics-addr", metricsAddr)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("fail to redirect stderr: %v", err)
	}

	done := make(chan struct{})
	go func(t *testing.T) {
		if err := waitForReady(stderr, done); err != nil {
			t.Fatalf(err.Error())
		}
	}(t)

	if err := cmd.Start(); err != nil {
		t.Fatalf("fail to start otelcontribcol: %v", err)
	}

	<-done
	return cmd, stderr
}

func waitForReady(stderr io.ReadCloser, done chan<- struct{}) error {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		nextLine := scanner.Text()

		if strings.Contains(nextLine, "Everything is ready.") {
			done <- struct{}{}
			return nil
		}

		if strings.Contains(nextLine, "Error:") {
			done <- struct{}{}
			return fmt.Errorf("collector fail: %v", nextLine)
		}
	}

	return fmt.Errorf("end of input reached without reading finish")
}

func startSampleApp(t *testing.T) *exec.Cmd {
	cmd := exec.Command("app/main")

	if err := cmd.Start(); err != nil {
		t.Fatalf("fail to start app: %v", err)
	}

	return cmd
}

func timeLogs(t *testing.T, stderr io.ReadCloser, numSamples, discard int) time.Duration {
	scanner := bufio.NewScanner(stderr)

	primeLogTimer(t, scanner, discard)

	var total time.Duration
	var prevTime time.Time
	for i := 0; i <= numSamples; {
		scanner.Scan()
		nextLine := scanner.Text()

		if strings.Contains(nextLine, "MetricsExporter") {
			timeStamp := strings.Fields(nextLine)[0]

			logTimeFmt := "2006-01-02T15:04:05.999-0700"
			timeObj, err := time.Parse(logTimeFmt, timeStamp)
			if err != nil {
				t.Errorf("fail to parse time: %v", timeStamp)
			}

			if !prevTime.IsZero() {
				total += timeObj.Sub(prevTime)
			}

			prevTime = timeObj
			i++
		}
	}

	return total / time.Duration(numSamples)

}

func primeLogTimer(t *testing.T, scanner *bufio.Scanner, discard int) {
	for {
		scanner.Scan()
		nextLine := scanner.Text()
		if strings.Contains(nextLine, "MetricsExporter") {
			if discard > 0 {
				discard--
			} else {
				return
			}
		} else if strings.Contains(nextLine, "error") {
			t.Fatalf("Fail to scan: %v", nextLine)
		}
	}
}

func fuzzyEqualDuration(first, second, tolerance time.Duration) bool {
	difference := float64(first - second)
	return math.Abs(difference) < float64(tolerance)
}

func getSchedulesFile(t *testing.T) *os.File {
	file, err := os.OpenFile("testdata/schedules.yaml", os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("fail to open schedules.yaml: %v", err)
	}

	return file
}

func writeString(t *testing.T, file *os.File, text string) {
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("cannot seek to beginning: %v", err)
	}

	if err := file.Truncate(0); err != nil {
		t.Fatalf("cannot truncate: %v", err)
	}

	if _, err := file.WriteString(text); err != nil {
		file.Close()
		t.Fatalf("cannot write schedule: %v", err)
	}
}
