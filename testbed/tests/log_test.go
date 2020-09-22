// Copyright 2019, OpenTelemetry Authors
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

package tests

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/testbed/testbed"
	scenarios "go.opentelemetry.io/collector/testbed/tests"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
)

func TestLog10kLPS(t *testing.T) {
	t.Run("Stanza", func(t *testing.T) {
		stanzaTests := []struct {
			name            string
			maxLinesPerFile int
			maxBackups      int
		}{
			{"NoRotation", 200000, 0},
			{"RotateEvery1000", 1000, 20},
		}

		for _, test := range stanzaTests {
			t.Run(test.name, func(t *testing.T) {
				port := testbed.GetAvailablePort(t)

				// Start a fake listener to work around the TestCase port liveness checks
				startListener(t, port)
				sls, err := datasenders.NewStanzaLogSender(tempDir(t), test.maxLinesPerFile, test.maxBackups, port)
				require.NoError(t, err)

				scenarios.Scenario10kItemsPerSecond(
					t,
					sls,
					testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
					testbed.ResourceSpec{
						ExpectedMaxCPU: 500,
						ExpectedMaxRAM: 500,
					},
					contribPerfResultsSummary,
					sls.Processors(),
					nil,
				)
			})
		}
	})
}

func startListener(t *testing.T, port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
				// handle error
			}
			time.Sleep(time.Second)
			conn.Close()
		}
	}()
	t.Cleanup(func() { wg.Wait() })
	t.Cleanup(func() { _ = ln.Close() })
}

func tempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}
